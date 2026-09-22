<template>
<form-layout class="edit-post-page page-width-md" title="Edit post">

	<loading-message v-if="loading"/>

	<template v-else>
		<form-field title="Post text" required>
			<el-input v-model="text" type="textarea" size="large" :maxlength="$const.postMaxLength"
				autocapitalize="sentences" :rows="6"/>
		</form-field>

		<form-actions>
			<el-button @click="submit()" type="primary" :disabled="submitDisabled">Save</el-button>
			<el-button @click="cancel()">Cancel</el-button>
		</form-actions>
	</template>

</form-layout>
</template>

<script>
import {
	ajaxGet,
	ajaxPost,
} from '@/utils/ajax.js';

import {
	alertSuccess,
} from '@/utils/notify.js';

export default {
	data() {
		return {
			post: null,
			text: '',
			loading: true,
			saving: false,
		};
	},
	computed: {
		postId() {
			return this.$route.params.id;
		},
		submitDisabled() {
			return this.saving || !this.text.trim();
		},
	},
	mounted() {
		this.load();
	},
	methods: {
		load() {
			this.loading = true;
			ajaxGet('/ajax/post', {id: this.postId}).then(response => {
				this.post = response.post || null;
				this.text = this.post ? this.post.postText : '';
			}).finally(() => {
				this.loading = false;
			});
		},
		submit() {
			if (this.submitDisabled) {
				return;
			}
			this.$confirm('Save changes?', 'Confirm', {
				confirmButtonText: 'Yes',
				cancelButtonText: 'No',
				type: 'warning',
			}).then(() => {
				this.saving = true;
				ajaxPost('/ajax/post/edit', {
					id: this.postId,
					text: this.text,
				}, {
					'invalid-post-text': 'The post text is invalid or too long.',
				}).then(() => {
					alertSuccess('Post updated.');
					this.$router.push({name: 'post', params: {id: this.postId}});
				}).finally(() => {
					this.saving = false;
				});
			}).catch(() => {
				// User cancelled
			});
		},
		cancel() {
			this.$router.push({name: 'post', params: {id: this.postId}});
		},
	},
};
</script>
