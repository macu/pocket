export const accountErrors = {
	'invalid-name': 'Enter a non-empty name containing only printable characters (maximum 50 characters).',
	'invalid-handle': 'The handle must start with a letter and contain only letters, numbers, and underscores (maximum 25 characters).',
	'handle-exists': 'That handle is already in use.',
	'invalid-email': 'Enter a valid email address (maximum 50 characters).',
	'email-exists': 'That email address is unavailable.',
	'incorrect-password': 'The old password is incorrect.',
	'invalid-password': 'The new password must meet the minimum length, be at most 72 bytes, and match the confirmation.',
	'invalid-token': 'This link is invalid or has already been used.',
	'token-expired': 'This link has expired. Please request a new one.',
	'email-rate-limit': 'Please wait a minute before requesting another confirmation email.',
	'profile-moderation-rejected': 'Your display name or handle was rejected by AI moderation. Please choose different profile details.',
};

export function normalizeName(name) {
	return name.trim().replace(/ +/g, ' ');
}

export function validName(name) {
	const normalized = normalizeName(name);
	return !!normalized && Array.from(normalized).length <= 50 && !/[\p{C}\p{Zl}\p{Zp}\p{Zs}]/u.test(name.replace(/ /g, ''));
}

export function validPassword(password, minLength) {
	return passwordValidationError(password, minLength) === '';
}

export function passwordValidationError(password, minLength) {
	if (!password.trim() || Array.from(password).length < minLength) {
		return `Password must contain at least ${minLength} characters and cannot be blank.`;
	}
	if (new TextEncoder().encode(password).length > 72) {
		return 'Password must be at most 72 bytes.';
	}
	return '';
}
