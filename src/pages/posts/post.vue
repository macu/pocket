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
				<topics-input ref="addTopicInput" v-model="newTopics" :max="$const.maxNewPostTopics"/>
			</form-field>
			<form-actions>
				<el-button @click="addTopic()" type="primary" :disabled="addTopicDisabled">Add topics</el-button>
				<el-button @click="toggleAddTopicForm()" type="default">Cancel</el-button>
			</form-actions>
		</form-layout>

		<template v-else>
			<horizontal-controls v-if="post && authenticated">
				<el-button v-if="isOwnPost" @click="goToEditPost()" type="primary">
					Edit
				</el-button>
				<el-button @click="toggleAddTopicForm()" type="primary">
					Add topics
				</el-button>
				<el-button @click="goToAddSubPost()" type="primary">
					Add sub-post
				</el-button>
			</horizontal-controls>

			<horizontal-controls class="align-start">

				<timeframe-select v-model="timeframe"/>

			</horizontal-controls>

			<h3>Top Topics in this Space</h3>

			<div v-if="selectedTopics.length > 0 || topTopics.length > 0" class="top-topics flex-row-md">
				<topic
					v-for="topic in selectedTopics"
					:key="'selected-' + topic.id"
					size="medium"
					checkable
					checked
					@check="uncheckTopic(topic)">
					{{topic.name}}
				</topic>
				<topic
					v-for="topic in topTopics"
					:key="topic.id"
					size="medium"
					:count="topic.postCount"
					count-output="%d posts"
					count-output-singular="%d post"
					checkable
					@check="checkTopic(topic)">
					{{topic.name}}
				</topic>
				<el-button v-if="showLoadMoreTopics"
					@click="loadMoreTopics()"
					type="primary" text size="small">
					Load More
				</el-button>
				<el-button v-if="selectedTopics.length > 0"
					@click="clearSelectedTopics()"
					type="warning" text size="small">
					Clear selected
				</el-button>
			</div>
			<p v-else><em>No topics available.</em></p>

			<h3>Top Sub-Posts</h3>

			<p v-if="topSubPosts.length > 0" class="total-posts">{{totalSubPosts}} matching posts</p>

			<div v-if="topSubPosts.length" class="top-sub-posts flex-column-md">
				<post-widget v-for="subPost in topSubPosts" :key="subPost.id" :post="subPost" clickable size="medium" @click="openPost(subPost.id)" />
				<el-button v-if="showLoadMoreSubPosts" @click="loadMoreSubPosts()" type="primary">
					Load More
				</el-button>
			</div>
			<p v-else><em>No sub-posts available.</em></p>

		</template>

	</template>
</div>
</template>

<script>
import PostWidget from '@/widgets/post.vue';
import TopicsInput from '@/widgets/topics-input.vue';
import TimeframeSelect, {TIMEFRAME_STORAGE_KEY} from '@/widgets/timeframe-select.vue';

import {
	ajaxGet,
	ajaxPost,
} from '@/utils/ajax.js';

import {
	getStorage,
	setStorage,
} from '@/utils/storage.js';

export default {
	components: {
		PostWidget,
		TopicsInput,
		TimeframeSelect,
	},
	data() {
		return {
			post: null,
			parentPost: null,
			topTopics: [],
			hasMoreTopics: true,
			selectedTopics: [],
			topSubPosts: [],
			totalSubPosts: 0,
			loading: true,
			newTopics: [],
			subPostText: '',
			showAddTopicForm: false,
			showAddSubPostForm: false,
			timeframe: getStorage(TIMEFRAME_STORAGE_KEY, '24h'),
		};
	},
	computed: {
		authenticated() {
			return this.$store.getters.authenticated;
		},
		isOwnPost() {
			return !!this.post && this.post.authorId === this.$store.getters.currentUserId;
		},
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
			return this.topSubPosts.length < this.totalSubPosts;
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
			this.selectedTopics = [];
			this.load();
		},
		timeframe(timeframe) {
			setStorage(TIMEFRAME_STORAGE_KEY, timeframe);
			this.load();
		},
	},
	methods: {
		load() {
			this.loading = true;
			ajaxGet('/ajax/post', {
				id: this.$route.params.id,
				loadContent: true,
				timeframe: this.timeframe,
			}).then(response => {
				this.post = response.post || null;
				this.parentPost = response.parentPost || null;
				this.topTopics = response.topTopics || [];
				this.hasMoreTopics = true;
				this.topSubPosts = response.topSubPosts || [];
				this.totalSubPosts = response.totalSubPosts || 0;
				if (this.selectedTopics.length > 0) {
					this.reloadFiltered();
				}
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
				timeframe: this.timeframe,
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
				timeframe: this.timeframe,
			}).then(response => {
				const posts = response.posts || [];
				this.topSubPosts.push(...posts);
				this.totalSubPosts = response.totalPosts || 0;
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
				timeframe: this.timeframe,
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
				timeframe: this.timeframe,
			}).then(response => {
				const posts = response.posts || [];
				this.topSubPosts = posts;
				this.totalSubPosts = response.totalPosts || 0;
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
		goToEditPost() {
			this.$router.push({name: 'edit-post', params: {id: this.post.id}});
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

	.total-posts {
		opacity: 0.7;
		font-size: 0.9em;
	}

	.post-card {
		border-top: thin solid white;
		border-bottom: thin solid white;
		padding: 10px 0;
	}
}
</style>
