package env

import "os"

// this package retrieves environment variables

func IsAppEngine() bool {
	return os.Getenv("APP_ENGINE") == "1"
}

func IsLocal() bool {
	return !IsAppEngine()
}

func GetMaintenanceMode() string {
	return os.Getenv("MAINTENANCE_MODE")
}

func GetPort() string {
	return os.Getenv("PORT")
}

func GetDbHost() string {
	return os.Getenv("DB_HOST")
}

func GetDbUser() string {
	return os.Getenv("DB_USER")
}

func GetDbPass() string {
	return os.Getenv("DB_PASS")
}

func GetDbPassSecret() string {
	return os.Getenv("DB_PASS_SECRET")
}

func GetDbName() string {
	return os.Getenv("DB_NAME")
}

func GetVersionStamp() string {
	return os.Getenv("VERSION_STAMP")
}

func GetDomain() string {
	return os.Getenv("DOMAIN")
}

func GetGoogleCloudProject() string {
	return os.Getenv("GOOGLE_CLOUD_PROJECT")
}

func GetCacheControlVersionStamp() string {
	return GetVersionStamp()
}

func GetRecaptchaKey() string {
	return os.Getenv("RECAPTCHA_KEY")
}

func GetMailjetApiKey() string {
	return os.Getenv("MAILJET_API_KEY")
}

func GetMailjetSecret() string {
	return os.Getenv("MAILJET_SECRET")
}

func GetMailjetFromEmail() string {
	return os.Getenv("MAILJET_FROM_EMAIL")
}
