<template>
<div class="dashboard-page flex-column-lg page-width-md">

	<return-to-top/>

	<horizontal-controls v-if="authenticated">

		<el-button @click="addTopic()" type="primary">
			Add Topic
		</el-button>

		<el-button @click="createPost()" type="primary">
			Create Post
		</el-button>

	</horizontal-controls>

	<div class="flex-column-lg">

		<template v-if="showingAddTopic">

			<form-layout class="add-topic-form" title="Add topic">

				<form-field title="Topic name">
					<el-input v-model="newTopicName" type="text" :maxlength="$const.maxTopicLength"
						autocapitalize="words"
						@keyup.enter.native="submitAddTopic()"
					/>
				</form-field>

				<form-actions>
					<el-button @click="submitAddTopic()" :disabled="addTopicDisabled" type="primary">
						Save Topic
					</el-button>
					<el-button @click="cancelAddTopic()" :disabled="cancelAddTopicDisabled">Cancel</el-button>
				</form-actions>

			</form-layout>

		</template>

		<loading-message v-else-if="loading"/>

		<template v-else>

			<h2>Top Topics</h2>

			<div v-if="selectedTopics.length > 0 || topTopics.length > 0" class="top-topics flex-row">
				<topic
					v-for="topic in selectedTopics"
					:key="'selected-' + topic.id"
					size="large"
					checkable
					checked
					@check="uncheckTopic(topic)">
					{{topic.name}}
				</topic>
				<topic
					v-for="topic in topTopics"
					:key="topic.id"
					size="large"
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

			<h2>Top Posts</h2>

			<p v-if="topPosts.length > 0" class="total-posts">{{totalPosts}} matching posts</p>

			<div v-if="topPosts.length > 0" class="top-posts flex-column-lg">
				<post
					v-for="post in topPosts"
					:key="post.id"
					:post="post"
					clickable
					:expandable="false"
					size="medium"
					@click="openPost(post.id)"
				/>

				<el-button v-if="showLoadMorePosts"
					@click="loadMorePosts()"
					type="primary" size="small">
					Load More
				</el-button>
			</div>

			<p v-else><em>No posts available.</em></p>

		</template>

	</div>

</div>
</template>

<script>
import Post from '@/widgets/post.vue';

import {
	ajaxGet,
	ajaxPost,
} from '@/utils/ajax.js';

import {
	alertSuccess,
	showError,
} from '@/utils/notify.js';

export default {
	components: {
		Post,
	},
	data() {
		return {
			loading: true,
			topPosts: [],
			totalPosts: 0,
			topTopics: [],
			hasMoreTopics: true,
			selectedTopics: [],

			showingAddTopic: false,
			newTopicName: '',
			addTopicLoading: false,
		};
	},
	computed: {
		authenticated() {
			return this.$store.getters.authenticated;
		},
		showLoadMoreTopics() {
			return this.hasMoreTopics &&
				this.topTopics.length > 0 &&
				this.topTopics.length % this.$const.maxTopicPageSize === 0;
		},
		showLoadMorePosts() {
			return this.topPosts.length < this.totalPosts;
		},
		addTopicDisabled() {
			return this.addTopicLoading || !this.newTopicName.trim();
		},
		cancelAddTopicDisabled() {
			return this.addTopicLoading;
		},
	},
	watch: {
		filter: {
			handler() {
				this.loadDashboard();
			},
			deep: true,
		},
	},
	mounted() {
		this.loadDashboard();
	},
	methods: {
		selectedTopicIds() {
			return this.selectedTopics.map(topic => topic.id).join(',');
		},
		loadDashboard() {
			this.loading = true;
			this.selectedTopics = [];
			ajaxGet('/ajax/dashboard').then(response => {
				this.topPosts = response.topPosts || [];
				this.totalPosts = response.totalPosts || 0;
				this.topTopics = response.topTopics || [];
				this.hasMoreTopics = true;
			}).finally(() => {
				this.loading = false;
			});
		},
		loadMoreTopics() {
			ajaxGet('/ajax/topics/page', {
				context: 'dashboard',
				offset: this.topTopics.length,
				topicIds: this.selectedTopicIds(),
			}).then(response => {
				const topics = response.topics || [];
				this.topTopics.push(...topics);
				this.hasMoreTopics = topics.length > 0;
			});
		},
		loadMorePosts() {
			ajaxGet('/ajax/posts/page', {
				context: 'dashboard',
				offset: this.topPosts.length,
				topicIds: this.selectedTopicIds(),
			}).then(response => {
				const posts = response.posts || [];
				this.topPosts.push(...posts);
				this.totalPosts = response.totalPosts || 0;
			});
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
				context: 'dashboard',
				offset: 0,
				topicIds,
			}).then(response => {
				const topics = response.topics || [];
				this.topTopics = topics;
				this.hasMoreTopics = topics.length > 0;
			});

			ajaxGet('/ajax/posts/page', {
				context: 'dashboard',
				offset: 0,
				topicIds,
			}).then(response => {
				const posts = response.posts || [];
				this.topPosts = posts;
				this.totalPosts = response.totalPosts || 0;
			});
		},

		addTopic() {
			this.showingAddTopic = true;
			this.newTopicName = '';
		},
		cancelAddTopic() {
			this.showingAddTopic = false;
			this.newTopicName = '';
			this.addTopicLoading = false;
		},
		submitAddTopic() {
			if (this.addTopicDisabled) {
				return;
			}
			this.addTopicLoading = true;
			ajaxPost('/ajax/topic', {
				name: this.newTopicName,
			}, {
				// special error codes
				'invalid-topic-name': 'The topic name provided is invalid.',
			}).then(response => {
				if (response.exists) {
					showError('That topic already exists.');
					return;
				}
				if (response.topic) {
					this.topTopics.unshift(response.topic);
				}
				this.showingAddTopic = false;
				this.newTopicName = '';
				alertSuccess('Topic added.');
			}).finally(() => {
				this.addTopicLoading = false;
			});
		},
		openPost(postId) {
			this.$router.push({name: 'post', params: {id: postId}});
		},
		createPost() {
			this.$router.push({name: 'add-post'});
		},
	},
};
</script>

<style lang="scss">
@import '@/styles/vars.scss';

.dashboard-page {
	color: $app-fg-color;

	>.horizontal-controls {
		padding: 10px;
		border-radius: $border-radius;
	}

	.add-topic-form {
		background-color: $topic-bg-color;
		color: $topic-fg-color;
	}

	.total-posts {
		opacity: 0.7;
		font-size: 0.9em;
	}

}
</style>
