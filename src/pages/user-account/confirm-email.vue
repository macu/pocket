<template>
<form-layout class="page-width-sm" title="Confirm email change">
	<template v-if="$route.query.token && !confirmed">
		<p>Confirm that this email address should be used for your Pocket account.</p>
		<form-actions>
			<el-button @click="confirm" :loading="saving" :disabled="saving">Confirm email</el-button>
		</form-actions>
	</template>
	<el-alert v-else-if="confirmed" title="Your email address has been updated." type="success" :closable="false"/>
	<el-alert v-else title="This confirmation link is invalid." type="error" :closable="false"/>
	<router-link v-if="$store.getters.authenticated" :to="{name: 'account'}">Account settings</router-link>
	<router-link v-else :to="{name: 'login'}">Log in</router-link>
</form-layout>
</template>

<script>
import {ajaxPost} from '@/utils/ajax.js';
import {accountErrors} from '@/utils/account.js';

export default {
	data() { return {saving: false, confirmed: false}; },
	methods: {
		confirm() {
			if (this.saving || this.confirmed) return;
			this.saving = true;
			ajaxPost('/ajax/account/confirm-email', {token: this.$route.query.token}, accountErrors).then(async () => {
				this.confirmed = true;
				await this.$router.replace({name: 'confirm-email'});
				await this.$store.dispatch('loadLogin');
			}).catch(() => {}).finally(() => { this.saving = false; });
		},
	},
};
</script>
