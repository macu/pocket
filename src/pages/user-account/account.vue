<template>
<form-layout class="page-width-sm" title="Account settings">
	<loading-message v-if="loading"/>
	<template v-else-if="loaded">
		<form-field title="Name" required>
			<el-input v-model="displayName" :maxlength="50" show-word-limit autocomplete="name"/>
			<p>Your name must contain only printable characters. Extra spaces are removed.</p>
		</form-field>
		<form-field title="Handle" required>
			<el-input v-model="handle" :maxlength="25" show-word-limit autocapitalize="none" autocomplete="off"
				:aria-invalid="!!handleError" :aria-describedby="handleError ? 'account-handle-error' : undefined"/>
			<p v-if="handleError" id="account-handle-error" class="form-field-error" role="alert">{{handleError}}</p>
			<p>Start with a letter, then use letters, numbers, or underscores. Must be unique.</p>
		</form-field>
		<form-field title="Email address" required>
			<el-input v-model="email" type="email" :maxlength="50" autocomplete="email" autocapitalize="none">
				<template #suffix>{{email.length}} / 50</template>
			</el-input>
			<p>A new address takes effect only after confirmation.</p>
			<p v-if="pendingEmail">Awaiting confirmation: {{pendingEmail}}. Your current address is {{currentEmail}}. Save your current address to cancel.</p>
		</form-field>
		<el-checkbox class="change-analytics" v-model="trackingDisabled" :disabled="saving">
			Disable analytics and tracking cookies
		</el-checkbox>
		<el-checkbox class="change-password" v-model="changePassword" @change="clearPasswords">Change password</el-checkbox>
		<template v-if="changePassword">
			<form-field title="Old password" required>
				<el-input v-model="oldPassword" type="password" show-password autocomplete="current-password"/>
			</form-field>
			<form-field title="New password" required>
				<p>Minimum length is {{$store.getters.passwordMinLength}} characters.</p>
				<el-input v-model="newPassword" type="password" show-password autocomplete="new-password"
					@blur="passwordTouched = true"
					:aria-invalid="!!passwordError" :aria-describedby="passwordError ? 'account-password-error' : undefined"/>
				<p v-if="passwordError" id="account-password-error" class="form-field-error" role="alert">{{passwordError}}</p>
			</form-field>
			<form-field title="Confirm new password" required>
				<el-input v-model="confirmPassword" type="password" show-password autocomplete="new-password"
					@focus="confirmPasswordTouched = false"
					@blur="confirmPasswordTouched = true"
					:aria-invalid="!!confirmPasswordError" :aria-describedby="confirmPasswordError ? 'account-confirm-password-error' : undefined"/>
				<p v-if="confirmPasswordError" id="account-confirm-password-error" class="form-field-error" role="alert">{{confirmPasswordError}}</p>
			</form-field>
		</template>
		<form-actions>
			<el-button @click="save" :disabled="saveDisabled" :loading="saving">Save</el-button>
		</form-actions>
	</template>
	<el-alert v-if="!loading && !loaded" title="Unable to load account settings. Please log in and try again." type="error" :closable="false"/>
</form-layout>
</template>

<script>
import {ajaxGet, ajaxPost} from '@/utils/ajax.js';
import {accountErrors, normalizeName, passwordValidationError, validName, validPassword} from '@/utils/account.js';
import {alertSuccess} from '@/utils/notify.js';

export default {
	data() {
		return {
			loading: true, loaded: false, saving: false,
			displayName: '', handle: '', email: '', currentEmail: '', pendingEmail: null,
			trackingDisabled: false,
			changePassword: false, oldPassword: '', newPassword: '', confirmPassword: '',
			passwordTouched: false, confirmPasswordTouched: false,
		};
	},
	computed: {
		passwordError() {
			return this.passwordTouched ? passwordValidationError(this.newPassword, this.$store.getters.passwordMinLength) : '';
		},
		confirmPasswordError() {
			return this.confirmPasswordTouched && this.newPassword !== this.confirmPassword ?
				'Passwords do not match.' : '';
		},
		handleValid() {
			return new RegExp(this.$const.handlePattern).test(this.handle.trim());
		},
		handleError() {
			return this.handle && !this.handleValid ? accountErrors['invalid-handle'] : '';
		},
		saveDisabled() {
			return this.saving || !validName(this.displayName) ||
				!this.handleValid ||
				!this.email.trim() ||
				(this.changePassword && (!this.oldPassword || !validPassword(this.newPassword, this.$store.getters.passwordMinLength) ||
					this.newPassword !== this.confirmPassword));
		},
	},
	mounted() {
		ajaxGet('/ajax/account').then(profile => {
			Object.assign(this, profile);
			this.trackingDisabled = window.localStorage.getItem('pocketTrackingDisabled') === 'true';
			this.currentEmail = profile.email;
			this.email = profile.pendingEmail || profile.email;
			this.loaded = true;
		}).catch(() => {}).finally(() => { this.loading = false; });
	},
	methods: {
		clearPasswords() {
			this.oldPassword = this.newPassword = this.confirmPassword = '';
			this.passwordTouched = this.confirmPasswordTouched = false;
		},
		save() {
			if (this.saveDisabled) return;
			const trackingPreferenceChanged = this.trackingDisabled !==
				(window.localStorage.getItem('pocketTrackingDisabled') === 'true');
			this.saving = true;
			ajaxPost('/ajax/account', {
				displayName: normalizeName(this.displayName), handle: this.handle.trim(), email: this.email.trim(),
				changePassword: this.changePassword,
				oldPassword: this.oldPassword, newPassword: this.newPassword, confirmPassword: this.confirmPassword,
			}, accountErrors).then(async response => {
				this.clearPasswords();
				this.changePassword = false;
				const bypassEmail = response.bypassEmail && response.token;
				if (bypassEmail) {
					// Running locally without Mailjet: confirm the email change directly
					await ajaxPost('/ajax/account/confirm-email', {token: response.token}, accountErrors);
				}
				await this.$store.dispatch('loadLogin');
				const profileRoute = {
					name: 'user',
					params: {identifier: this.$store.getters.currentUserHandle || String(this.$store.getters.currentUserId)},
				};
				if (trackingPreferenceChanged) {
					window.localStorage.setItem('pocketTrackingDisabled', String(this.trackingDisabled));
				}
				await this.$router.push(profileRoute);
				if (trackingPreferenceChanged) {
					window.location.reload();
					return;
				}
				alertSuccess(response.emailPending && !bypassEmail ?
					'Saved. Check your new email address for a confirmation link.' : 'Account settings saved.');
			}).catch(() => {}).finally(() => { this.saving = false; });
		},
	},
};
</script>

<style lang="scss" scoped>
.change-analytics, .change-password {
	margin-left: 10px;
}
</style>
