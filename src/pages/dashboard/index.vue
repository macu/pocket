<template>
<div class="dashboard-page flex-column-lg" :class="isMobile ? 'page-width-md' : 'page-width-xl'">

	<return-to-top/>

	<div class="flex-column-lg">

		<template v-if="showingAddTopic">

			<form-layout class="add-topic-form" title="Create topic">

				<form-field title="Topic name">
					<template #tip><small>Create a topic for others to use on their posts.</small></template>
					<el-input v-model="newTopicName" type="text" :maxlength="$const.maxTopicLength"
						autocapitalize="words"
						@input="scheduleTopicSearch()"
						@keyup.enter.native="submitAddTopic()"
					/>
					<div v-if="topicSearchLoading || matchingTopics.length > 0" class="matching-topics">
						<small>Existing topics:</small>
						<p v-if="topicSearchLoading"><em>Searching...</em></p>
						<ul v-else>
							<li v-for="topic in matchingTopics" :key="topic.id">{{topic.name}}</li>
						</ul>
					</div>
				</form-field>

				<form-actions>
					<el-button @click="submitAddTopic()" :disabled="addTopicDisabled" type="primary">
						Create
					</el-button>
					<el-button @click="cancelAddTopic()" :disabled="cancelAddTopicDisabled">
						Cancel
					</el-button>
				</form-actions>

			</form-layout>

		</template>

		<loading-message v-else-if="loading"/>

		<div v-else class="dashboard-columns" :class="{'is-mobile': isMobile}">

			<div ref="columnLeft" class="dashboard-column-left flex-column-lg">

				<horizontal-controls class="align-start">

					<div class="flex-column-sm">
						<small>Find posts created within:</small>
						<timeframe-select v-model="timeframe"/>
					</div>

				</horizontal-controls>

				<h2>Top topics</h2>

				<horizontal-controls v-if="authenticated || !topicSelectionLimitReached" class="align-start">
					<el-button v-if="authenticated" @click="addTopic()" type="primary">
						Create topic
					</el-button>
					<el-button v-if="!topicSelectionLimitReached" @click="toggleTopicSearch()" type="primary">
						Search topics
					</el-button>
				</horizontal-controls>

				<div v-if="showTopicSearch && !topicSelectionLimitReached" class="dashboard-topic-search">
					<topic-search
						v-model="topicSearchQuery"
						:exclude-ids="selectedTopics.map(topic => topic.id)"
						@select="selectSearchTopic"
					/>
				</div>

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
					<template v-if="!topicSelectionLimitReached">
						<topic
							v-for="topic in uniqueTopTopics"
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
							Load more
						</el-button>
					</template>
					<el-button v-if="selectedTopics.length > 0"
						@click="clearSelectedTopics()"
						type="warning" text size="small">
						Clear selected
					</el-button>
				</div>

				<p v-else class="no-topics"><em>No topics available.</em></p>

			</div>

			<div class="dashboard-column-right flex-column-lg">

				<h2>Top posts</h2>

				<horizontal-controls v-if="authenticated" class="align-start">
					<el-button @click="createPost()" type="primary">
						Create post
					</el-button>
				</horizontal-controls>

				<p v-if="topPosts.length > 0" class="total-posts">{{totalPosts}} matching posts</p>

				<div v-if="topPosts.length > 0" class="top-posts flex-column-lg">
					<post
						v-for="post in uniqueTopPosts"
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
						Load more
					</el-button>
				</div>

				<p v-else class="no-sub-posts"><em>No posts available.</em></p>

			</div>

		</div>

	</div>

</div>
</template>

<script>
import Post from '@/widgets/post.vue';
import TopicSearch from '@/widgets/topic-search.vue';
import TimeframeSelect, {TIMEFRAME_RECENT, TIMEFRAME_STORAGE_KEY} from '@/widgets/timeframe-select.vue';

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

import {PRESELECTED_TOPICS_STORAGE_KEY} from '@/pages/posts/add-post.vue';

const SELECTED_TOPICS_STORAGE_KEY = 'dashboard.selectedTopics';

export default {
	components: {
		Post,
		TopicSearch,
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
			showTopicSearch: false,
			topicSearchQuery: '',
			matchingTopics: [],
			topicSearchLoading: false,
			topicSearchTimeout: null,
			topicSearchRequestId: 0,
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
			return !this.topicSelectionLimitReached &&
				this.hasMoreTopics &&
				this.topTopics.length > 0 &&
				this.topTopics.length % this.$const.maxTopicPageSize === 0;
		},
		showLoadMorePosts() {
			return this.topPosts.length < this.totalPosts;
		},
		topicSelectionLimitReached() {
			return this.selectedTopics.length >= this.$const.maxTopicSelectionCount;
		},
		// dedupe by id since pagination offsets can shift as votes reorder results between page loads
		uniqueTopTopics() {
			const seen = new Set();
			return this.topTopics.filter(topic => (seen.has(topic.id) ? false : seen.add(topic.id)));
		},
		uniqueTopPosts() {
			const seen = new Set();
			return this.topPosts.filter(post => (seen.has(post.id) ? false : seen.add(post.id)));
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
		selectSearchTopic(topic) {
			this.topicSearchQuery = '';
			this.showTopicSearch = false;
			this.checkTopic(topic);
		},
		toggleTopicSearch() {
			this.showTopicSearch = !this.showTopicSearch;
			if (!this.showTopicSearch) {
				this.topicSearchQuery = '';
			}
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
			if (this.topicSelectionLimitReached) {
				return;
			}
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
			const params = {
				context: 'dashboard',
				offset: this.topPosts.length,
				topicIds: this.selectedTopicIds(),
				timeframe: this.timeframe,
			};
			if (this.timeframe === TIMEFRAME_RECENT && this.topPosts.length > 0) {
				const lastPost = this.topPosts[this.topPosts.length - 1];
				params.offset = 0;
				params.createdAt = lastPost.createdAt;
				params.cursorPostId = lastPost.id;
			}
			ajaxGet('/ajax/posts/page', params).then(response => {
				const posts = response.posts || [];
				this.topPosts.push(...posts);
				this.totalPosts = response.totalPosts || 0;
			});
		},
		checkTopic(topic) {
			if (this.topicSelectionLimitReached) {
				return;
			}
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

			// once the selection limit is reached, don't load any more co-topics to choose from
			let topicsPromise = Promise.resolve();
			if (this.topicSelectionLimitReached) {
				this.topTopics = [];
				this.hasMoreTopics = false;
			} else {
				topicsPromise = ajaxGet('/ajax/topics/page', {
					context: 'dashboard',
					offset: 0,
					topicIds,
					timeframe: this.timeframe,
				}).then(response => {
					const topics = response.topics || [];
					this.topTopics = topics;
					this.hasMoreTopics = topics.length > 0;
				});
			}

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
			this.clearTopicSearch();
		},
		scheduleTopicSearch() {
			clearTimeout(this.topicSearchTimeout);
			const requestId = ++this.topicSearchRequestId;
			const query = this.newTopicName.trim();
			this.matchingTopics = [];
			this.topicSearchLoading = Boolean(query);
			if (!query) {
				return;
			}
			this.topicSearchTimeout = setTimeout(() => {
				ajaxGet('/ajax/topics/search', {query}).then(response => {
					if (requestId === this.topicSearchRequestId) {
						this.matchingTopics = response.topics || [];
					}
				}).catch(() => {
					// ajaxGet already displays a request error.
				}).finally(() => {
					if (requestId === this.topicSearchRequestId) {
						this.topicSearchLoading = false;
					}
				});
			}, 250);
		},
		clearTopicSearch() {
			clearTimeout(this.topicSearchTimeout);
			this.topicSearchRequestId++;
			this.matchingTopics = [];
			this.topicSearchLoading = false;
		},
		cancelAddTopic() {
			this.clearTopicSearch();
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
				alertSuccess('Topic added. Thanks!');
			}).finally(() => {
				this.addTopicLoading = false;
			});
		},
		openPost(postId) {
			this.$router.push({name: 'post', params: {id: postId}});
		},
		createPost() {
			setStorage(PRESELECTED_TOPICS_STORAGE_KEY, this.selectedTopics.map(topic => topic.name));
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

	.total-posts, .no-topics, .no-sub-posts {
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
