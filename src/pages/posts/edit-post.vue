<template>
<form-layout class="edit-post-page page-width-md" title="Edit post">

	<loading-message v-if="loading"/>

	<template v-else-if="post">
		<form-field title="Post text">
			<p>Add text, an image, or both.</p>
			<el-input v-model="text" type="textarea" size="large" :maxlength="$const.postMaxLength"
				autocapitalize="sentences" :autosize="{minRows: 6}"/>
		</form-field>

		<form-field title="Image">
			<post-image-input v-model="image" v-model:remove-current="removeImage"
				:current-image="post.imageUrl || ''" :disabled="saving || deleting"/>
		</form-field>

		<form-actions>
			<el-button @click="submit()" type="primary" :disabled="submitDisabled">Save</el-button>
			<el-button @click="cancel()" :disabled="saving || deleting">Cancel</el-button>
			<el-button v-if="isOwnPost" @click="deletePost()" type="danger" :disabled="saving || deleting" :loading="deleting">Delete</el-button>
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
import PostImageInput from '@/widgets/post-image-input.vue';
import {postErrors} from '@/utils/post.js';

export default {
	components: {PostImageInput},
	data() {
		return {
			post: null,
			text: '',
			image: null,
			removeImage: false,
			loading: true,
			saving: false,
			deleting: false,
		};
	},
	computed: {
		postId() {
			return this.$route.params.id;
		},
		submitDisabled() {
			return this.saving || this.deleting ||
				(!this.text.trim() && !this.image && !(this.post?.imageUrl && !this.removeImage));
		},
		isOwnPost() {
			return !!this.post && this.post.authorId === this.$store.getters.currentUserId;
		},
	},
	mounted() {
		this.load();
	},
	methods: {
		async deletePost() {
			if (!this.isOwnPost || this.saving || this.deleting) {
				return;
			}
			try {
				await this.$confirm('Delete this post, including its sub-posts, votes and revisions? This cannot be undone.', 'Confirm delete', {
					confirmButtonText: 'Delete',
					cancelButtonText: 'Cancel',
					type: 'warning',
				});
			} catch (action) {
				if (action === 'cancel' || action === 'close') {
					return;
				}
				throw action;
			}
			this.deleting = true;
			let response;
			try {
				response = await ajaxPost('/ajax/post/delete', {id: this.postId});
			} catch (error) {
				// ajaxPost displays the failure; leave the post available for retry.
				return;
			} finally {
				this.deleting = false;
			}
			alertSuccess('Post deleted.');
			await this.$router.push(response.parentPostId ?
				{name: 'post', params: {id: response.parentPostId}} : {name: 'dashboard'});
		},
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
					image: this.image,
					removeImage: this.removeImage,
				}, postErrors).then(() => {
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
