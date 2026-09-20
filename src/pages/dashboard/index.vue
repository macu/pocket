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
					<el-input v-model="newTopicName" type="text" maxlength="50"
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
				<topic v-for="topic in topTopics" :key="topic.id" size="large" :count="topic.sum" :negative="topic.sum < 0">
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
			topTopics: [],
			lastTopicsLength: 0,
			lastPostsLength: 0,

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
			return this.lastTopicsLength > 0;
		},
		showLoadMorePosts() {
			return this.lastPostsLength > 0;
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
				this.topTopics = response.topTopics || [];
				this.lastTopicsLength = this.topTopics.length;
				this.lastPostsLength = this.topPosts.length;
			}).finally(() => {
				this.loading = false;
			});
		},
		loadMoreTopics() {
		},
		loadMorePosts() {
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
