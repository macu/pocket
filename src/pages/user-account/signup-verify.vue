<template>
<form-layout class="signup-verify-page page-width-sm">

	<template #title>Verify sign up</template>

	<loading-message v-if="loading" message="Loading signup request..."/>

	<template v-else-if="signupRequest">

		<p>Finish setting up your account below.</p>

		<form-field title="Email address" required>
			<el-input v-model="signupRequest.email" type="email"
				readonly autocomplete="username"/>
		</form-field>

		<form-field title="Handle" required>
			<p>This handle will make your user profile accessible via URL.</p>
			<p>Start with a letter, then use letters, numbers, or underscores, with no spaces.</p>
			<el-input v-model="handle" type="text"
				:maxlength="25" show-word-limit
				autocapitalize="none" autocomplete="off"
				:aria-invalid="!!handleError" :aria-describedby="handleError ? 'signup-handle-error' : undefined"/>
			<p v-if="handleError" id="signup-handle-error" class="form-field-error" role="alert">{{handleError}}</p>
		</form-field>

		<form-field title="Display name" required>
			<label></label>
			<p>This name will be displayed on your user profile and posts you create. Use printable characters only. Extra spaces are removed.</p>
			<el-input v-model="displayName" type="text"
				:maxlength="50" show-word-limit
				autocapitalize="none" autocomplete="off"/>
		</form-field>

		<form-field title="Password">
			<p>Minimum length is {{$store.getters.passwordMinLength}} characters; maximum is 72 bytes.</p>
			<el-input v-model="password" type="password"
				:maxlength="100" show-password
				autocapitalize="none" autocomplete="new-password"
				@blur="passwordTouched = true"
				:aria-invalid="!!passwordError" :aria-describedby="passwordError ? 'signup-password-error' : undefined"/>
			<p v-if="passwordError" id="signup-password-error" class="form-field-error" role="alert">{{passwordError}}</p>
		</form-field>

		<form-field title="Verify password">
			<el-input v-model="verifyPassword" type="password"
				:maxlength="100" show-password
				autocapitalize="none" autocomplete="new-password"
				@focus="confirmPasswordTouched = false"
				@blur="confirmPasswordTouched = true"
				:aria-invalid="!!confirmPasswordError" :aria-describedby="confirmPasswordError ? 'signup-confirm-password-error' : undefined"/>
			<p v-if="confirmPasswordError" id="signup-confirm-password-error" class="form-field-error" role="alert">{{confirmPasswordError}}</p>
		</form-field>

		<form-field title="Message to admin (optional; say Hi)">
			<el-input v-model="message" type="textarea" :maxlength="200"
				:autosize="{minRows: 2}" show-word-limit
				autocapitalize="none" autocomplete="off"/>
		</form-field>

		<form-actions>
			<el-button @click="create()" :disabled="createDisabled">Create account</el-button>
		</form-actions>

		<loading-message v-if="submitting" message="Creating account..."/>

	</template>

	<el-alert v-else title="Invalid request" type="error" show-icon :closable="false">
		<p>The signup request is invalid or expired.</p>
		<p>Please <router-link :to="{name: 'signup'}">register again</router-link>.</p>
	</el-alert>

</form-layout>
</template>

<script>
import store from '@/store.js';

import {
	ajaxGet,
	ajaxPost,
} from '@/utils/ajax.js';
import {accountErrors, normalizeName, passwordValidationError, validName, validPassword} from '@/utils/account.js';

export default {
	data() {
		return {
			loading: true,
			signupRequest: null,

			password: '',
			verifyPassword: '',
			passwordTouched: false,
			confirmPasswordTouched: false,
			handle: '',
			displayName: '',
			message: '',
			submitting: false,
		};
	},
	computed: {
		passwordError() {
			return this.passwordTouched ? passwordValidationError(this.password, this.$store.getters.passwordMinLength) : '';
		},
		confirmPasswordError() {
			return this.confirmPasswordTouched && this.password !== this.verifyPassword ?
				'Passwords do not match.' : '';
		},
		handleValid() {
			return new RegExp(this.$const.handlePattern).test(this.handle.trim());
		},
		handleError() {
			return this.handle && !this.handleValid ? accountErrors['invalid-handle'] : '';
		},
		createDisabled() {
			return this.loading || !this.signupRequest ||
				this.submitting || !validName(this.displayName) ||
				!this.handleValid ||
				!validPassword(this.password, this.$store.getters.passwordMinLength) ||
				this.password !== this.verifyPassword;
		},
	},
	beforeRouteEnter(to, from, next) {
		if (store.getters.authenticated) {
			next({name: 'dashboard'});
		} else {
			next(vm => {
				if (vm.$route.query.token) {
					vm.loadSignupRequest(vm.$route.query.token);
				} else {
					vm.loading = false;
				}
			});
		}
	},
	methods: {
		loadSignupRequest(token) {
			ajaxGet('/ajax/load-signup', {
				token,
			}, {
				'invalid-token': 'Invalid token.',
				'token-expired': 'This signup request has expired.',
			}).then(response => {
				this.signupRequest = response;
				this.loading = false;
			}).catch(() => {
				this.loading = false;
			});
		},
		create() {
			if (this.createDisabled) {
				return;
			}
			this.submitting = true;
			ajaxPost('/ajax/signup-verify', {
				token: this.signupRequest.token.trim(),
				password: this.password,
				handle: this.handle.trim(),
				displayName: normalizeName(this.displayName),
				message: this.message.trim(),
			}, {
				...accountErrors,
				'invalid-token': 'Invalid token.',
				'token-expired': 'This signup request has expired.',
				'invalid-name': 'Enter a non-empty display name containing only printable characters (maximum 50 characters).',
				'invalid-handle': 'The handle must start with a letter and contain only letters, numbers, and underscores (maximum 25 characters).',
				'handle-exists': 'The given handle already exists.',
				'email-exists': 'A user with the given email address already exists.',
			}).then(() => {
				this.$store.dispatch('loadLogin');
				this.$router.replace({
					name: 'dashboard',
				});
			}).catch(() => {
				this.submitting = false;
			});
		},
	},
};
</script>
