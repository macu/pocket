<template>
<div class="dashboard-page flex-column-lg" :class="isMobile ? 'page-width-md' : 'page-width-xl'">

	<return-to-top/>

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

		<div v-else class="dashboard-columns" :class="{'is-mobile': isMobile}">

			<div ref="columnLeft" class="dashboard-column-left flex-column-lg">

				<horizontal-controls class="align-start">

					<timeframe-select v-model="timeframe"/>

				</horizontal-controls>

				<h2>Top Topics</h2>

				<horizontal-controls v-if="authenticated" class="align-start">
					<el-button @click="addTopic()" type="primary">
						Add Topic
					</el-button>
				</horizontal-controls>

				<div v-if="selectedTopics.length > 0 || topTopics.length > 0" ref="topicsList" class="top-topics flex-row">
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

			</div>

			<div class="dashboard-column-right flex-column-lg">

				<h2>Top Posts</h2>

				<horizontal-controls v-if="authenticated" class="align-start">
					<el-button @click="createPost()" type="primary">
						Create Post
					</el-button>
				</horizontal-controls>

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

			</div>

		</div>

	</div>

</div>
</template>

<script>
import Post from '@/widgets/post.vue';
import TimeframeSelect, {TIMEFRAME_STORAGE_KEY} from '@/widgets/timeframe-select.vue';

import {
	ajaxGet,
	ajaxPost,
} from '@/utils/ajax.js';

import {
	alertSuccess,
	showError,
} from '@/utils/notify.js';

import {
	getStorage,
	setStorage,
} from '@/utils/storage.js';

const SELECTED_TOPICS_STORAGE_KEY = 'dashboard.selectedTopics';

export default {
	components: {
		Post,
		TimeframeSelect,
	},
	data() {
		return {
			loading: true,
			timeframe: getStorage(TIMEFRAME_STORAGE_KEY, '24h'),
			topPosts: [],
			totalPosts: 0,
			topTopics: [],
			hasMoreTopics: true,
			selectedTopics: getStorage(SELECTED_TOPICS_STORAGE_KEY, []),

			showingAddTopic: false,
			newTopicName: '',
			addTopicLoading: false,
		};
	},
	computed: {
		authenticated() {
			return this.$store.getters.authenticated;
		},
		isMobile() {
			return this.$store.getters.isMobile;
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
		timeframe(timeframe) {
			setStorage(TIMEFRAME_STORAGE_KEY, timeframe);
			this.loadDashboard();
		},
		selectedTopics(selectedTopics) {
			setStorage(SELECTED_TOPICS_STORAGE_KEY, selectedTopics);
		},
	},
	mounted() {
		if (this.selectedTopics.length > 0) {
			this.loading = true;
			this.reloadFiltered().finally(() => {
				this.loading = false;
			});
		} else {
			this.loadDashboard();
		}
	},
	methods: {
		selectedTopicIds() {
			return this.selectedTopics.map(topic => topic.id).join(',');
		},
		loadDashboard() {
			this.loading = true;
			ajaxGet('/ajax/dashboard', {
				timeframe: this.timeframe,
			}).then(response => {
				this.topPosts = response.topPosts || [];
				this.totalPosts = response.totalPosts || 0;
				this.topTopics = response.topTopics || [];
				this.hasMoreTopics = true;
				if (this.selectedTopics.length > 0) {
					this.reloadFiltered();
				}
			}).finally(() => {
				this.loading = false;
			});
		},
		loadMoreTopics() {
			ajaxGet('/ajax/topics/page', {
				context: 'dashboard',
				offset: this.topTopics.length,
				topicIds: this.selectedTopicIds(),
				timeframe: this.timeframe,
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
				timeframe: this.timeframe,
			}).then(response => {
				const posts = response.posts || [];
				this.topPosts.push(...posts);
				this.totalPosts = response.totalPosts || 0;
			});
		},
		checkTopic(topic) {
			this.topTopics = this.topTopics.filter(t => t.id !== topic.id);
			// reassign (rather than push) so the selectedTopics watcher fires and persists the change
			this.selectedTopics = [...this.selectedTopics, topic];
			this.reloadFiltered().then(() => this.resetColumnsScroll());
		},
		uncheckTopic(topic) {
			this.selectedTopics = this.selectedTopics.filter(t => t.id !== topic.id);
			this.reloadFiltered().then(() => this.resetColumnsScroll());
		},
		clearSelectedTopics() {
			this.selectedTopics = [];
			this.reloadFiltered().then(() => this.resetColumnsScroll());
		},
		resetColumnsScroll() {
			// todo
		},
		reloadFiltered() {
			const topicIds = this.selectedTopicIds();

			const topicsPromise = ajaxGet('/ajax/topics/page', {
				context: 'dashboard',
				offset: 0,
				topicIds,
				timeframe: this.timeframe,
			}).then(response => {
				const topics = response.topics || [];
				this.topTopics = topics;
				this.hasMoreTopics = topics.length > 0;
			});

			const postsPromise = ajaxGet('/ajax/posts/page', {
				context: 'dashboard',
				offset: 0,
				topicIds,
				timeframe: this.timeframe,
			}).then(response => {
				const posts = response.posts || [];
				this.topPosts = posts;
				this.totalPosts = response.totalPosts || 0;
			});

			return Promise.all([topicsPromise, postsPromise]);
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

	.dashboard-columns {
		display: flex;
		align-items: flex-start;
		column-gap: 40px;

		>.dashboard-column-left {
			flex: 0 0 300px;
			position: sticky;
			top: 20px;
			max-height: calc(100vh - 40px);
			overflow-y: auto;
			padding: 5px;
		}

		>.dashboard-column-right {
			flex: 1;
			min-width: 0;
			padding: 5px;
		}

		&.is-mobile {
			flex-direction: column;
			row-gap: 40px;

			>.dashboard-column-left {
				flex-basis: auto;
				position: static;
				max-height: none;
				overflow-y: visible;
			}
		}
	}

}
</style>
