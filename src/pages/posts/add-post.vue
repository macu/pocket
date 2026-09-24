<template>
<form-layout class="add-post-page page-width-md" title="Add post">

	<form-field v-if="parentPost" title="Replying to">
		<div class="parent-post-preview">
			<post-widget :post="parentPost" />
		</div>
	</form-field>

	<form-field title="Post text" required>
		<el-input v-model="text" type="textarea" size="large" :maxlength="$const.postMaxLength"
			autocapitalize="sentences" :rows="6"/>
	</form-field>

	<form-field title="Topics">
		<topics-input v-model="topics" :max="$const.maxNewPostTopics"/>
	</form-field>

	<form-actions>
		<el-button @click="submit()" type="primary" :disabled="submitDisabled">Post</el-button>
		<el-button @click="cancel()">Cancel</el-button>
	</form-actions>

</form-layout>
</template>

<script>
import PostWidget from '@/widgets/post.vue';
import TopicsInput from '@/widgets/topics-input.vue';

import {
	ajaxGet,
	ajaxPost,
} from '@/utils/ajax.js';

export default {
	components: {
		PostWidget,
		TopicsInput,
	},
	data() {
		return {
			text: '',
			topics: [],
			parentId: null,
			parentPost: null,
			loading: false,
		};
	},
	computed: {
		submitDisabled() {
			return this.loading || !this.text.trim();
		},
	},
	mounted() {
		this.parentId = this.$route.query.parentId ? Number(this.$route.query.parentId) : null;
		if (this.parentId) {
			this.loadParentPost();
		}
	},
	methods: {
		loadParentPost() {
			this.loading = true;
			ajaxGet('/ajax/post', {id: this.parentId}).then(response => {
				this.parentPost = response.post || null;
			}).finally(() => {
				this.loading = false;
			});
		},
		submit() {
			if (this.submitDisabled) {
				return;
			}
			this.loading = true;
			ajaxPost('/ajax/post', {
				text: this.text,
				topics: JSON.stringify(this.topics),
				parentId: this.parentId,
			}, {
				'invalid-post-text': 'The post text is invalid or too long.',
			}).then(response => {
				const newPostId = response && response.post ? response.post.id : null;
				if (newPostId) {
					this.$router.push({name: 'post', params: {id: newPostId}});
					return;
				}
				this.$router.push({name: 'dashboard'});
			}).finally(() => {
				this.loading = false;
			});
		},
		cancel() {
			this.$router.push({name: 'dashboard'});
		},
	},
};
</script>

<style lang="scss">
@import '@/styles/vars.scss';

.add-post-page {
	color: $app-fg-color;

	.parent-post-preview {
		border-radius: $border-radius;
		background: rgba(255, 255, 255, 0.04);
		white-space: pre-wrap;
		word-break: break-word;
	}
}
</style>
