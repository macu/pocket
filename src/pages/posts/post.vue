<template>
<div class="post-page page-width-md flex-column-lg">

	<loading-message v-if="loading"/>

	<template v-else-if="post">

		<div v-if="parentPost" class="parent-post-card flex-column-sm">
			<small>Parent post</small>
			<post-widget :post="parentPost" clickable @click="openPost(parentPost.id)" size="small" />
		</div>

		<div class="post-card">
			<post-widget :post="post" default-expanded size="large" />
		</div>

		<form-layout v-if="showAddTopicForm" title="Add topics" class="add-topic-form">
			<form-field title="Topic names">
				<topics-input ref="addTopicInput" v-model="newTopics"/>
			</form-field>
			<form-actions>
				<el-button @click="addTopic()" type="primary" :disabled="addTopicDisabled">Add topics</el-button>
				<el-button @click="toggleAddTopicForm()" type="default">Cancel</el-button>
			</form-actions>
		</form-layout>

		<template v-else>
			<horizontal-controls v-if="post">
				<el-button @click="toggleAddTopicForm()" type="primary">
					Add topics
				</el-button>
				<el-button @click="goToAddSubPost()" type="primary">
					Add sub-post
				</el-button>
			</horizontal-controls>

			<h3>Top Topics in this Space</h3>

			<div v-if="selectedTopics.length > 0 || topTopics.length > 0" class="top-topics flex-row-md">
				<topic
					v-for="topic in selectedTopics"
					:key="'selected-' + topic.id"
					size="medium"
					:count="topic.sum"
					checkable
					checked
					@check="uncheckTopic(topic)"
				>
					{{topic.name}}
				</topic>
				<topic
					v-for="topic in topTopics"
					:key="topic.id"
					size="medium"
					:count="topic.sum"
					checkable
					@check="checkTopic(topic)"
				>
					{{topic.name}}
				</topic>
				<el-button v-if="showLoadMoreTopics" @click="loadMoreTopics()" type="primary">
					Load More
				</el-button>
				<el-button v-if="selectedTopics.length > 0" @click="clearSelectedTopics()">
					Clear selected
				</el-button>
			</div>
			<p v-else>No topics available.</p>

			<h3>Top Sub-Posts</h3>

			<div v-if="topSubPosts.length" class="top-sub-posts">
				<post-widget v-for="subPost in topSubPosts" :key="subPost.id" :post="subPost" clickable size="medium" @click="openPost(subPost.id)" />
				<el-button v-if="showLoadMoreSubPosts" @click="loadMoreSubPosts()" type="primary">
					Load More
				</el-button>
			</div>
			<p v-else>No sub-posts available.</p>

		</template>

	</template>
</div>
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
			post: null,
			parentPost: null,
			topTopics: [],
			hasMoreTopics: true,
			selectedTopics: [],
			topSubPosts: [],
			hasMoreSubPosts: true,
			loading: true,
			newTopics: [],
			subPostText: '',
			showAddTopicForm: false,
			showAddSubPostForm: false,
		};
	},
	computed: {
		addTopicDisabled() {
			return this.newTopics.length === 0;
		},
		addSubPostDisabled() {
			return !this.subPostText.trim();
		},
		showLoadMoreTopics() {
			return this.hasMoreTopics &&
				this.topTopics.length > 0 &&
				this.topTopics.length % this.$const.maxTopicPageSize === 0;
		},
		showLoadMoreSubPosts() {
			return this.hasMoreSubPosts &&
				this.topSubPosts.length > 0 &&
				this.topSubPosts.length % this.$const.maxPostPageSize === 0;
		},
		postId() {
			return this.$route.params.id;
		},
	},
	mounted() {
		this.load();
	},
	watch: {
		postId() {
			this.load();
		},
	},
	methods: {
		load() {
			this.loading = true;
			this.selectedTopics = [];
			ajaxGet('/ajax/post', {
				id: this.$route.params.id,
				loadContent: true,
			}).then(response => {
				this.post = response.post || null;
				this.parentPost = response.parentPost || null;
				this.topTopics = response.topTopics || [];
				this.hasMoreTopics = true;
				this.topSubPosts = response.topSubPosts || [];
				this.hasMoreSubPosts = true;
			}).finally(() => {
				this.loading = false;
			});
		},

		focusAddTopicInput() {
			this.$nextTick(() => {
				if (this.$refs.addTopicInput && this.$refs.addTopicInput.focus) {
					this.$refs.addTopicInput.focus();
				}
			});
		},
		toggleAddTopicForm() {
			this.showAddTopicForm = !this.showAddTopicForm;
			if (this.showAddTopicForm) {
				this.focusAddTopicInput();
			}
		},
		loadMoreTopics() {
			ajaxGet('/ajax/topics/page', {
				context: 'space',
				postId: this.post.id,
				offset: this.topTopics.length,
				topicIds: this.selectedTopicIds(),
			}).then(response => {
				const topics = response.topics || [];
				this.topTopics.push(...topics);
				this.hasMoreTopics = topics.length > 0;
			});
		},
		loadMoreSubPosts() {
			ajaxGet('/ajax/posts/page', {
				context: 'subposts',
				postId: this.post.id,
				offset: this.topSubPosts.length,
				topicIds: this.selectedTopicIds(),
			}).then(response => {
				const posts = response.posts || [];
				this.topSubPosts.push(...posts);
				this.hasMoreSubPosts = posts.length > 0;
			});
		},
		selectedTopicIds() {
			return this.selectedTopics.map(topic => topic.id).join(',');
		},
		checkTopic(topic) {
			this.topTopics = this.topTopics.filter(t => t.id !== topic.id);
			this.selectedTopics.push(topic);
			this.reloadFiltered();
		},
		uncheckTopic(topic) {
			this.selectedTopics = this.selectedTopics.filter(t => t.id !== topic.id);
			this.reloadFiltered();
		},
		clearSelectedTopics() {
			this.selectedTopics = [];
			this.reloadFiltered();
		},
		reloadFiltered() {
			const topicIds = this.selectedTopicIds();

			ajaxGet('/ajax/topics/page', {
				context: 'space',
				postId: this.post.id,
				offset: 0,
				topicIds,
			}).then(response => {
				const topics = response.topics || [];
				this.topTopics = topics;
				this.hasMoreTopics = topics.length > 0;
			});

			ajaxGet('/ajax/posts/page', {
				context: 'subposts',
				postId: this.post.id,
				offset: 0,
				topicIds,
			}).then(response => {
				const posts = response.posts || [];
				this.topSubPosts = posts;
				this.hasMoreSubPosts = posts.length > 0;
			});
		},

		openPost(postId) {
			this.$router.push({name: 'post', params: {id: postId}});
		},
		addTopic() {
			if (this.addTopicDisabled) {
				return;
			}
			ajaxPost('/ajax/post/topic', {
				postId: this.post.id,
				topics: JSON.stringify(this.newTopics),
			}).then(response => {
				this.newTopics = [];
				this.post = response.post || this.post;
				this.showAddTopicForm = false;
			});
		},
		goToAddSubPost() {
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

	.add-topic-form {
		background-color: $topic-bg-color;
		color: $topic-fg-color;
	}
}
</style>
