<template>
<div class="post-page page-width-md flex-column-lg">

	<loading-message v-if="loading"/>

	<template v-else-if="post">

		<template v-if="parentPost">
			<div class="parent-post-card">
				<small>Parent post</small>
				<post-widget :post="parentPost" />
			</div>
		</template>

		<div class="post-card">
			<post-widget :post="post" default-expanded />
		</div>

		<form-layout v-if="showAddTagForm" title="Add tag">
			<form-field title="Tag name">
				<el-input ref="addTagInput" v-model="newTag" type="text" maxlength="50" @keyup.enter.native="addTag()"/>
			</form-field>
			<form-actions>
				<el-button @click="addTag()" type="primary" :disabled="addTagDisabled">Add tag</el-button>
				<el-button @click="toggleAddTagForm()" type="default">Cancel</el-button>
			</form-actions>
		</form-layout>

		<form-layout v-else-if="showAddSubPostForm" title="Add sub-post">
			<form-field title="Sub-post text" required>
				<el-input ref="addPostInput" v-model="subPostText" type="textarea" :maxlength="$const.postMaxLength" :rows="4"/>
			</form-field>
			<form-actions>
				<el-button @click="addSubPost()" :disabled="addSubPostDisabled" type="primary">Add sub-post</el-button>
				<el-button @click="toggleAddSubPostForm()" type="default">Cancel</el-button>
			</form-actions>
		</form-layout>

		<template v-else>
			<horizontal-controls v-if="post">
				<el-button @click="toggleAddTagForm()" type="primary">
					{{showAddTagForm ? 'Hide tag form' : 'Add tag'}}
				</el-button>
				<el-button @click="toggleAddSubPostForm()" type="primary">
					{{showAddSubPostForm ? 'Hide sub-post form' : 'Add sub-post'}}
				</el-button>
			</horizontal-controls>
		</template>

	</template>
</div>
</template>

<script>
import PostWidget from '@/widgets/post.vue';

import {
	ajaxGet,
	ajaxPost,
} from '@/utils/ajax.js';

export default {
	components: {
		PostWidget,
	},
	data() {
		return {
			post: null,
			parentPost: null,
			loading: true,
			newTag: '',
			subPostText: '',
			showAddTagForm: false,
			showAddSubPostForm: false,
		};
	},
	computed: {
		addTagDisabled() {
			return !this.newTag.trim();
		},
		addSubPostDisabled() {
			return !this.subPostText.trim();
		},
	},
	mounted() {
		this.load();
	},
	methods: {
		focusAddTagInput() {
			this.$nextTick(() => {
				if (this.$refs.addTagInput && this.$refs.addTagInput.focus) {
					this.$refs.addTagInput.focus();
				}
			});
		},
		focusAddPostInput() {
			this.$nextTick(() => {
				if (this.$refs.addPostInput && this.$refs.addPostInput.focus) {
					this.$refs.addPostInput.focus();
				}
			});
		},
		toggleAddTagForm() {
			this.showAddTagForm = !this.showAddTagForm;
			if (this.showAddTagForm) {
				this.focusAddTagInput();
			}
		},
		toggleAddSubPostForm() {
			this.showAddSubPostForm = !this.showAddSubPostForm;
			if (this.showAddSubPostForm) {
				this.focusAddPostInput();
			}
		},
		load() {
			this.loading = true;
			ajaxGet('/ajax/post', {
				id: this.$route.params.id,
			}).then(response => {
				this.post = response.post || null;
				this.parentPost = response.parentPost || null;
			}).finally(() => {
				this.loading = false;
			});
		},
		addTag() {
			if (!this.newTag.trim()) {
				return;
			}
			ajaxPost('/ajax/post/tag', {
				postId: this.post.id,
				tag: this.newTag,
			}).then(response => {
				this.newTag = '';
				this.post = response.post || this.post;
			});
		},
		addSubPost() {
			if (!this.subPostText.trim()) {
				return;
			}
			this.$router.push({
				name: 'add-post',
				query: {parentId: this.post.id},
			});
		},
	},
};
</script>

<style lang="scss">
@import '@/styles/vars.scss';

.post-page {
	color: $app-fg-color;
}
</style>
