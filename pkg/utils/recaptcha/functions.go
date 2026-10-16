package recaptcha

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"google.golang.org/api/recaptchaenterprise/v1"

	"pocket/pkg/env"
	"pocket/pkg/utils/net"
)

const DefaultMinScore = 0.5

type VerificationError struct {
	Score  *float64
	Reason string
	cause  error
}

func (e *VerificationError) Error() string {
	return e.cause.Error()
}

func (e *VerificationError) Unwrap() error {
	return e.cause
}

var newAssessmentService = func(ctx context.Context) (*recaptchaenterprise.Service, error) {
	return recaptchaenterprise.NewService(ctx)
}

func ActionForPath(path string) string {
	return strings.NewReplacer("/", "_", "-", "_").Replace(strings.TrimPrefix(path, "/ajax/"))
}

func extractToken(r *http.Request) string {
	token := r.Header.Get("X-Recaptcha-Token")
	if token == "" {
		token = r.FormValue("g-recaptcha-response")
	}
	if token == "" {
		token = r.FormValue("recaptcha_token")
	}
	return token
}

func Verify(r *http.Request, expectedAction ...string) (*recaptchaenterprise.GoogleCloudRecaptchaenterpriseV1Assessment, error) {
	token := extractToken(r)
	if token == "" {
		return nil, fmt.Errorf("reCAPTCHA token undefined")
	}
	key := env.GetRecaptchaKey()
	if key == "" {
		return nil, fmt.Errorf("reCAPTCHA key undefined")
	}
	project := env.GetGoogleCloudProject()
	if project == "" {
		return nil, fmt.Errorf("reCAPTCHA Google Cloud project undefined")
	}
	action := ActionForPath(r.URL.Path)
	if len(expectedAction) > 0 && expectedAction[0] != "" {
		action = expectedAction[0]
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	service, err := newAssessmentService(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating reCAPTCHA Enterprise client: %w", err)
	}
	assessment, err := service.Projects.Assessments.Create(
		"projects/"+project,
		&recaptchaenterprise.GoogleCloudRecaptchaenterpriseV1Assessment{
			Event: &recaptchaenterprise.GoogleCloudRecaptchaenterpriseV1Event{
				Token:          token,
				SiteKey:        key,
				ExpectedAction: action,
				UserIpAddress:  net.GetUserIP(r),
				UserAgent:      r.UserAgent(),
			},
		},
	).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("creating reCAPTCHA Enterprise assessment: %w", err)
	}
	return assessment, nil
}

func VerifyRecaptcha(r *http.Request, expectedAction ...string) (bool, error) {
	return VerifyRecaptchaWithMinScore(r, DefaultMinScore, expectedAction...)
}

func VerifyRecaptchaWithMinScore(r *http.Request, minScore float64, expectedAction ...string) (bool, error) {
	if math.IsNaN(minScore) || minScore < 0 || minScore > 1 {
		return false, fmt.Errorf("invalid reCAPTCHA score threshold")
	}
	if env.IsLocal() && env.GetRecaptchaKey() == "" {
		return true, nil
	}
	assessment, err := Verify(r, expectedAction...)
	if err != nil {
		return false, &VerificationError{Reason: "assessment_failed", cause: err}
	}
	var score *float64
	if assessment.RiskAnalysis != nil && !math.IsNaN(assessment.RiskAnalysis.Score) && !math.IsInf(assessment.RiskAnalysis.Score, 0) {
		score = &assessment.RiskAnalysis.Score
	}
	fail := func(reason string, err error) (bool, error) {
		return false, &VerificationError{Score: score, Reason: reason, cause: err}
	}
	action := ActionForPath(r.URL.Path)
	if len(expectedAction) > 0 && expectedAction[0] != "" {
		action = expectedAction[0]
	}
	if assessment.TokenProperties == nil || !assessment.TokenProperties.Valid {
		reason := "invalid_token"
		if assessment.TokenProperties != nil && assessment.TokenProperties.InvalidReason != "" {
			reason += ":" + assessment.TokenProperties.InvalidReason
		}
		return fail(reason, fmt.Errorf("invalid reCAPTCHA token: %s", reason))
	}
	if assessment.TokenProperties.Action != action {
		return fail("action_mismatch", fmt.Errorf("reCAPTCHA action mismatch"))
	}
	if assessment.RiskAnalysis == nil {
		return fail("missing_risk_analysis", fmt.Errorf("reCAPTCHA risk analysis missing"))
	}
	if score == nil || *score < 0 || *score > 1 {
		return fail("invalid_score", fmt.Errorf("reCAPTCHA score invalid"))
	}
	if *score < minScore {
		return fail("low_score", fmt.Errorf("reCAPTCHA score %g below threshold %g", *score, minScore))
	}
	return true, nil
}
