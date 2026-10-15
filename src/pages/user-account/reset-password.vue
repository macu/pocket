<template>
<form-layout class="page-width-sm" title="Reset password">
	<template v-if="$route.query.token">
		<form-field title="New password" required>
			<p>Minimum length is {{$store.getters.passwordMinLength}} characters; maximum is 72 bytes.</p>
			<el-input v-model="newPassword" type="password" show-password autocomplete="new-password"/>
		</form-field>
		<form-field title="Confirm new password" required>
			<el-input v-model="confirmPassword" type="password" show-password autocomplete="new-password"/>
		</form-field>
		<form-actions>
			<el-button @click="reset" :loading="saving" :disabled="resetDisabled">Save new password</el-button>
		</form-actions>
	</template>
	<template v-else>
		<p>Enter your account email address to request a password reset link.</p>
		<form-field title="Email address" required>
			<el-input v-model="email" type="email" :maxlength="50" autocomplete="email" autocapitalize="none"/>
		</form-field>
		<form-actions>
			<el-button @click="request" :disabled="saving || !email.trim()" :loading="saving">Send reset link</el-button>
		</form-actions>
		<el-alert v-if="sent" title="If an active account exists for this address, a reset link has been sent. Check your email." type="success" :closable="false"/>
	</template>
	<router-link :to="{name: 'login'}">Log in</router-link>
</form-layout>
</template>

<script>
import {ajaxPost} from '@/utils/ajax.js';
import {accountErrors, validPassword} from '@/utils/account.js';

export default {
	data() { return {email: '', newPassword: '', confirmPassword: '', saving: false, sent: false}; },
	computed: {
		resetDisabled() {
			return this.saving || !validPassword(this.newPassword, this.$store.getters.passwordMinLength) ||
				this.newPassword !== this.confirmPassword;
		},
	},
	methods: {
		request() {
			if (this.saving || !this.email.trim()) return;
			this.saving = true;
			ajaxPost('/ajax/password-reset/request', {email: this.email.trim()}, accountErrors).then(async response => {
				this.sent = true;
				if (response.bypassEmail && response.token) {
					// Running locally without Mailjet: skip the email step
					await this.$router.replace({name: 'reset-password', query: {token: response.token}});
				}
			}).catch(() => {}).finally(() => { this.saving = false; });
		},
		reset() {
			if (this.resetDisabled) return;
			this.saving = true;
			ajaxPost('/ajax/password-reset', {
				token: this.$route.query.token, newPassword: this.newPassword, confirmPassword: this.confirmPassword,
			}, accountErrors).then(async () => {
				this.newPassword = this.confirmPassword = '';
				await this.$store.dispatch('loadLogin');
				await this.$router.replace({name: 'dashboard'});
			}).catch(() => {}).finally(() => { this.saving = false; });
		},
	},
};
</script>
