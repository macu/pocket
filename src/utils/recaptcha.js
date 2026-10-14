const failureMessage = 'reCAPTCHA verification failed.';
const timeoutMs = 10000;
let enterpriseReady;

function loadEnterprise(key) {
	if (!enterpriseReady) {
		enterpriseReady = new Promise((resolve, reject) => {
			const script = document.createElement('script');
			const timer = setTimeout(() => {
				script.remove();
				reject(new Error(failureMessage));
			}, timeoutMs);
			script.src = 'https://www.google.com/recaptcha/enterprise.js?render=' + encodeURIComponent(key);
			script.async = true;
			script.onerror = () => {
				clearTimeout(timer);
				script.remove();
				reject(new Error(failureMessage));
			};
			script.onload = () => {
				if (!window.grecaptcha?.enterprise) {
					clearTimeout(timer);
					script.remove();
					reject(new Error(failureMessage));
					return;
				}
				window.grecaptcha.enterprise.ready(() => {
					clearTimeout(timer);
					resolve(window.grecaptcha.enterprise);
				});
			};
			document.head.appendChild(script);
		}).catch(error => {
			enterpriseReady = undefined;
			throw error;
		});
	}
	return enterpriseReady;
}

export async function requestRecaptchaToken(url) {
	const key = window.appConstants.recaptchaKey;
	if (!key) {
		throw new Error(failureMessage);
	}
	const enterprise = await loadEnterprise(key);
	const path = new URL(url, window.location.href).pathname;
	const action = path.replace(/^\/ajax\//, '').replace(/[/-]/g, '_');
	return new Promise((resolve, reject) => {
		const timer = setTimeout(() => reject(new Error(failureMessage)), timeoutMs);
		Promise.resolve().then(() => enterprise.execute(key, {action})).then(token => {
			clearTimeout(timer);
			if (!token) {
				reject(new Error(failureMessage));
				return;
			}
			resolve(token);
		}).catch(error => {
			clearTimeout(timer);
			reject(new Error(failureMessage, {cause: error}));
		});
	});
}
