<template>
<div class="top-post" :class="{clickable}" @click="$emit('click', $event)">
	<div class="top-post-header flex-row-md">
		<div class="top-post-author" v-if="post.authorDisplayName">{{post.authorDisplayName}}</div>
		<div class="top-post-score">{{post.totalTopicScore}}</div>
	</div>
	<div class="topic-list flex-row-sm">
		<topic v-for="topic in post.topics" :key="topic.id" size="small" :count="topic.sum" votable :user-vote="topic.userVote" @vote="vote(topic, $event)">
			{{topic.name}}
		</topic>
	</div>
	<div class="top-post-text" :class="{expanded}" @click.stop="expanded = !expanded">{{post.postText}}</div>
</div>
</template>

<script>
import {ajaxPost} from '@/utils/ajax.js';

export default {
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
		defaultExpanded: {
			type: Boolean,
			default: false,
		},
	},
	data() {
		return {
			expanded: this.defaultExpanded,
		};
	},
	methods: {
		vote(topic, voteType) {
			ajaxPost('/ajax/post/topic/vote', {
				postId: this.post.id,
				topicId: topic.id,
				voteType,
			}).then(response => {
				if (response.topic) {
					Object.assign(topic, response.topic);
				}
				if (response.totalTopicScore !== undefined) {
					this.post.totalTopicScore = response.totalTopicScore;
				}
			});
		},
	},
};
</script>

<style lang="scss">
.top-post {
	padding: 12px;
	border-radius: 10px;
	background-color: rgba(255, 255, 255, 0.04);
	border: thin solid rgba(255, 255, 255, 0.15);
	display: flex;
	flex-direction: column;
	row-gap: 10px;

	&.clickable {
		cursor: pointer;
	}

	.top-post-header {
		justify-content: space-between;
		align-items: center;
		.top-post-author {
			font-weight: bold;
		}
		.top-post-score {
			padding: 2px 8px;
			border-radius: 10px;
			background-color: rgba(86, 86, 211, 0.2);
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

	.topic-list {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
	}
}
</style>
