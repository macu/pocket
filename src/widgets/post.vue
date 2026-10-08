<template>
<div class="top-post" :class="[sizeClass, {clickable}]" @click="$emit('click', $event)">
	<div class="top-post-header flex-row-md">
		<span class="top-post-votes" @click.stop>
			<span v-if="authenticated" class="vote-buttons">
				<material-icon
					v-if="!post.userVote || post.userVote === 'upvote'"
					icon="thumb_up" class="vote-btn upvote"
					:fill="post.userVote === 'upvote'"
					:class="{active: post.userVote === 'upvote'}"
					@click="votePost('upvote')"/>
				<material-icon
					v-if="!post.userVote || post.userVote === 'downvote'"
					icon="thumb_down" class="vote-btn downvote"
					:fill="post.userVote === 'downvote'"
					:class="{active: post.userVote === 'downvote'}"
					@click="votePost('downvote')"/>
			</span>
			<el-tooltip content="Post votes" placement="top">
				<span class="top-post-score">{{post.sum}}</span>
			</el-tooltip>
		</span>
		<div class="top-post-author" v-if="post.authorDisplayName">
			<router-link :to="{name: 'user', params: {identifier: post.authorHandle || String(post.authorId)}}" class="top-post-author-link" @click.stop>
				{{post.authorDisplayName}}
				<small v-if="post.authorHandle">@{{post.authorHandle}}</small>
			</router-link>
		</div>
		<small v-if="post.createdAt">
			&emsp;{{post.parentPostId ? 'subpost posted' : 'posted'}}
			<moment :time="post.createdAt" ago/>
		</small>
		<small v-if="post.subPostCount !== undefined">
			&emsp;{{post.subPostCount}} {{post.subPostCount === 1 ? 'subpost' : 'subposts'}}
		</small>
	</div>
	<div v-if="showTopics" class="topic-list flex-row">
		<topic v-for="topic in post.topics" :key="topic.id" :size="size" :count="topic.sum" votable :user-vote="topic.userVote" :author-upvoted="topic.authorUpvoted" @vote="vote(topic, $event)">
			{{topic.name}}
		</topic>
		<el-button v-if="showLoadMoreTopics" @click.stop="loadMoreTopics()" type="primary" size="small">
			Load More
		</el-button>
	</div>
	<div class="top-post-text" :class="{expanded}" @click.stop="toggleExpanded($event)">{{post.postText}}</div>
</div>
</template>

<script>
import {ajaxGet, ajaxPost} from '@/utils/ajax.js';
import MaterialIcon from '@/widgets/material-icon.vue';

export default {
	components: {
		MaterialIcon,
	},
	emits: ['click'],
	props: {
		post: {
			type: Object,
			required: true,
		},
		clickable: {
			type: Boolean,
			default: false,
		},
		expandable: {
			type: Boolean,
			default: true,
		},
		defaultExpanded: {
			type: Boolean,
			default: false,
		},
		size: {
			type: String,
			default: 'small',
			validator: value => ['small', 'medium', 'large'].includes(value),
		},
	},
	data() {
		return {
			expanded: this.defaultExpanded,
			hasMoreTopics: true,
		};
	},
	computed: {
		authenticated() {
			return this.$store.getters.authenticated;
		},
		sizeClass() {
			return 'size-' + this.size;
		},
		showTopics() {
			return this.post.topics && this.post.topics.length > 0;
		},
		showLoadMoreTopics() {
			return this.hasMoreTopics &&
				this.post.topics &&
				this.post.topics.length > 0 &&
				this.post.topics.length %
					this.$const.maxTopicPageSize === 0;
		},
	},
	methods: {
		loadMoreTopics() {
			ajaxGet('/ajax/topics/page', {
				context: 'post',
				postId: this.post.id,
				offset: this.post.topics.length,
			}).then(response => {
				const topics = response.topics || [];
				this.post.topics.push(...topics);
				this.hasMoreTopics = topics.length > 0;
			});
		},
		votePost(voteType) {
			ajaxPost('/ajax/post/vote', {
				postId: this.post.id,
				voteType,
			}).then(response => {
				this.post.sum = response.sum;
				this.post.userVote = response.userVote;
			});
		},
		vote(topic, voteType) {
			ajaxPost('/ajax/post/topic/vote', {
				postId: this.post.id,
				topicId: topic.id,
				voteType,
			}).then(response => {
				if (response.topic) {
					Object.assign(topic, response.topic);
				}
			});
		},
		toggleExpanded(event) {
			if (!this.expandable) {
				if (this.clickable) {
					this.$emit('click', event);
				}
				return;
			}
			if (!this.expanded) {
				this.expanded = true;
			} else {
				this.expanded = false;
			}
		},
	},
};
</script>

<style lang="scss">
@import '@/styles/vars.scss';

.top-post {
	padding: 12px;
	border-radius: 10px;
	background-color: rgba(255, 255, 255, 0.1);
	border: thin solid rgba(255, 255, 255, 0.15);
	display: flex;
	flex-direction: column;
	row-gap: 10px;

	.top-post-header {
		align-items: center;
		.top-post-author {
			flex: 1;
			font-weight: bold;
		}
		.top-post-author-link {
			color: inherit;
			text-decoration: none;
			&:hover {
				text-decoration: underline;
			}
		}
		.top-post-votes {
			display: inline-flex;
			align-items: center;
			gap: 6px;

			.vote-buttons {
				display: inline-flex;
				align-items: center;
				gap: 4px;
			}
			.vote-btn {
				cursor: pointer;
				font-size: 26px;
				opacity: 0.6;
				border-radius: 999px;
				&:hover {
					opacity: 0.85;
				}
				&.active {
					opacity: 1;
					color: white;
					padding: 4px;
					background-color: rgb(76, 175, 80);
					&.downvote {
						background-color: rgb(211, 86, 86);
					}
				}
			}
		}
		.top-post-score {
			padding: 2px 8px;
			border-radius: 10px;
			background-color: $el-dropdown-fg-color;
			color: $el-focus-color;
			font-weight: bold;
		}
	}

	.top-post-text {
		cursor: pointer;
		white-space: pre-wrap;
		word-break: break-word;
		display: -webkit-box;
		-webkit-box-orient: vertical;
		-webkit-line-clamp: 5;
		line-clamp: 5;
		overflow: hidden;
		&.expanded {
			-webkit-line-clamp: unset;
			line-clamp: unset;
		}
	}

	&.clickable {
		cursor: pointer;
	}

	&.size-small {
		padding: 8px;
		row-gap: 6px;
		font-size: 0.85em;

		.top-post-header {
			.top-post-score {
				padding: 2px 8px;
				border-radius: 10px;
			}
		}
	}

	&.size-medium {
		padding: 12px;
		row-gap: 10px;
		font-size: 1em;

		.top-post-header {
			.top-post-score {
				padding: 3px 10px;
				border-radius: 11px;
			}
		}
	}

	&.size-large {
		padding: 20px;
		row-gap: 24px;
		font-size: 1.3em;

		.top-post-header {
			.top-post-score {
				padding: 4px 12px;
				border-radius: 12px;
			}
		}
	}

}
</style>
