<template>
<div class="dashboard-page flex-column-lg page-width-md">

	<return-to-top/>

	<horizontal-controls v-if="loginLoaded">

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

			<div v-if="topTopics.length > 0" class="top-topics flex-row-lg">
				<topic v-for="topic in topTopics" :key="topic.id" size="large" :count="topic.sum">
					{{topic.name}}
				</topic>
				<el-button v-if="showLoadMoreTopics" @click="loadMoreTopics()" type="primary">
					Load More
				</el-button>
			</div>

			<p v-else>No topics available.</p>

			<h2>Top Posts</h2>

			<div v-if="topPosts.length > 0" class="top-posts flex-column-lg">
				<post
					v-for="post in topPosts"
					:key="post.id"
					:post="post"
					clickable
					size="medium"
					@click="openPost(post.id)"
				/>

				<el-button v-if="showLoadMorePosts" @click="loadMorePosts()" type="primary">
					Load More
				</el-button>
			</div>

			<p v-else>No posts available.</p>

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
			hasMorePosts: true,
			topTopics: [],
			hasMoreTopics: true,

			showingAddTopic: false,
			newTopicName: '',
			addTopicLoading: false,
		};
	},
	computed: {
		loginLoaded() {
			return this.$store.getters.loginLoaded;
		},
		showLoadMoreTopics() {
			return this.hasMoreTopics &&
				this.topTopics.length > 0 &&
				this.topTopics.length % this.$const.maxTopicPageSize === 0;
		},
		showLoadMorePosts() {
			return this.hasMorePosts &&
				this.topPosts.length > 0 &&
				this.topPosts.length % this.$const.maxPostPageSize === 0;
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
		loadDashboard() {
			this.loading = true;
			ajaxGet('/ajax/dashboard').then(response => {
				this.topPosts = response.topPosts || [];
				this.hasMorePosts = true;
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
			}).then(response => {
				const posts = response.posts || [];
				this.topPosts.push(...posts);
				this.hasMorePosts = posts.length > 0;
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

}
</style>
