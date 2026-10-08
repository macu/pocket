<template>
<div class="user-page flex-column-lg" :class="isMobile ? 'page-width-md' : 'page-width-xl'">

	<return-to-top/>

	<loading-message v-if="loading"/>

	<p v-else-if="!user" class="no-user"><em>User not found.</em></p>

	<div v-else class="user-columns" :class="{'is-mobile': isMobile}">

		<div class="user-column-left flex-column-lg">

			<div class="user-heading flex-column-sm">
				<h2>{{user.displayName}}</h2>
				<small v-if="user.handle">@{{user.handle}}</small>
				<small>Joined <moment :time="user.createdAt" ago/></small>
			</div>

			<horizontal-controls class="align-start">
				<div class="flex-column-sm">
					<small>Find posts created within:</small>
					<timeframe-select v-model="timeframe"/>
				</div>
			</horizontal-controls>

			<h3>Top topics</h3>

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

		<div class="user-column-right flex-column-lg">

			<h2>Posts</h2>

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

			<p v-else class="no-posts"><em>No posts available.</em></p>

		</div>

	</div>

</div>
</template>

<script>
import Post from '@/widgets/post.vue';
import TimeframeSelect, {TIMEFRAME_RECENT, TIMEFRAME_STORAGE_KEY} from '@/widgets/timeframe-select.vue';

import {
	ajaxGet,
} from '@/utils/ajax.js';

import {
	getStorage,
	setStorage,
} from '@/utils/storage.js';

export default {
	components: {
		Post,
		TimeframeSelect,
	},
	data() {
		return {
			loading: true,
			user: null,
			timeframe: getStorage(TIMEFRAME_STORAGE_KEY, '24h'),
			topPosts: [],
			totalPosts: 0,
			topTopics: [],
			hasMoreTopics: true,
			selectedTopics: [],
		};
	},
	computed: {
		isMobile() {
			return this.$store.getters.isMobile;
		},
		identifier() {
			return this.$route.params.identifier;
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
	},
	watch: {
		identifier() {
			this.selectedTopics = [];
			this.load();
		},
		timeframe(timeframe) {
			setStorage(TIMEFRAME_STORAGE_KEY, timeframe);
			this.load();
		},
	},
	mounted() {
		this.load();
	},
	methods: {
		selectedTopicIds() {
			return this.selectedTopics.map(topic => topic.id).join(',');
		},
		load() {
			this.loading = true;
			ajaxGet('/ajax/user', {
				user: this.identifier,
				timeframe: this.timeframe,
			}).then(response => {
				this.user = response.user || null;
				this.topPosts = response.topPosts || [];
				this.totalPosts = response.totalPosts || 0;
				this.topTopics = response.topTopics || [];
				this.hasMoreTopics = true;
				if (this.selectedTopics.length > 0) {
					return this.reloadFiltered();
				}
			}).catch(() => {
				this.user = null;
			}).finally(() => {
				this.loading = false;
			});
		},
		loadMoreTopics() {
			if (this.topicSelectionLimitReached) {
				return;
			}
			ajaxGet('/ajax/topics/page', {
				context: 'user',
				userId: this.user.id,
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
				context: 'user',
				userId: this.user.id,
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
			this.selectedTopics = [...this.selectedTopics, topic];
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

			// once the selection limit is reached, don't load any more co-topics to choose from
			let topicsPromise = Promise.resolve();
			if (this.topicSelectionLimitReached) {
				this.topTopics = [];
				this.hasMoreTopics = false;
			} else {
				topicsPromise = ajaxGet('/ajax/topics/page', {
					context: 'user',
					userId: this.user.id,
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
				context: 'user',
				userId: this.user.id,
				offset: 0,
				topicIds,
				timeframe: this.timeframe,
			}).then(response => {
				this.topPosts = response.posts || [];
				this.totalPosts = response.totalPosts || 0;
			});

			return Promise.all([topicsPromise, postsPromise]);
		},
		openPost(id) {
			this.$router.push({name: 'post', params: {id}});
		},
	},
};
</script>

<style lang="scss">
@import '@/styles/vars.scss';

.user-page {
	color: $app-fg-color;

	.total-posts, .no-topics, .no-posts, .no-user {
		opacity: 0.7;
		font-size: 0.9em;
	}

	.user-heading small {
		opacity: 0.7;
	}

	.user-columns {
		display: flex;
		align-items: flex-start;
		column-gap: 40px;

		>.user-column-left {
			flex: 0 0 300px;
			position: sticky;
			top: 20px;
			max-height: calc(100vh - 40px);
			overflow-y: auto;
			padding: 5px;
		}

		>.user-column-right {
			flex: 1;
			min-width: 0;
			padding: 5px;
		}

		&.is-mobile {
			flex-direction: column;
			row-gap: 40px;

			>.user-column-left {
				flex-basis: auto;
				position: static;
				max-height: none;
				overflow-y: visible;
			}
		}
	}
}
</style>
