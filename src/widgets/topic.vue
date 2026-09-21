<template>
<div class="topic" :class="[sizeClass, {negative: negative}]">
	<span class="topic-name"><slot/></span>
	<span v-if="showCount" class="topic-count">{{count}}</span>
	<span v-if="votable" class="topic-votes" @click.stop>
		<material-icon
			v-if="!userVote || userVote === 'upvote'"
			icon="thumb_up" class="vote-btn"
			:fill="userVote === 'upvote'"
			:class="{active: userVote === 'upvote'}"
			@click="$emit('vote', 'upvote')"/>
		<material-icon
			v-if="!userVote || userVote === 'downvote'"
			icon="thumb_down" class="vote-btn"
			:fill="userVote === 'downvote'"
			:class="{active: userVote === 'downvote'}"
			@click="$emit('vote', 'downvote')"/>
	</span>
</div>
</template>

<script>
import MaterialIcon from '@/widgets/material-icon.vue';

export default {
	components: {
		MaterialIcon,
	},
	emits: ['vote'],
	props: {
		size: {
			type: String,
			default: 'medium',
			validator(value) {
				return ['small', 'medium', 'large'].includes(value);
			},
		},
		count: {
			type: Number,
			default: null,
		},
		negative: {
			type: Boolean,
			default: false,
		},
		votable: {
			type: Boolean,
			default: false,
		},
		userVote: {
			type: String,
			default: null,
			validator(value) {
				return value === null || ['upvote', 'downvote'].includes(value);
			},
		},
	},
	computed: {
		sizeClass() {
			return 'size-' + this.size;
		},
		showCount() {
			return this.count !== null && this.count !== undefined;
		},
	},
	methods: {
		setVote(vote) {
			this.$emit('vote', vote);
		},
		undoVote() {
			this.$emit('vote', null);
		},
	},
};
</script>

<style lang="scss">
.topic {
	display: inline-flex;
	align-items: center;
	gap: 8px;
	padding: 10px 12px;
	border-radius: 10px;
	background-color: rgb(86, 86, 211);
	color: white;
	line-height: 1.2;
	white-space: nowrap;

	&.negative {
		background-color: rgb(211, 86, 86);
	}

	&.size-small {
		padding: 4px 8px;
		border-radius: 999px;
		font-size: 0.85em;
		gap: 6px;
	}

	&.size-large {
		padding: 10px;
		font-size: 1rem;
	}

	.topic-name {
		display: inline-block;
	}

	.topic-count {
		font-weight: bold;
		background-color: rgba(255, 255, 255, 0.1);
		padding: 2px 6px;
		border-radius: 6px;
	}

	.topic-votes {
		display: inline-flex;
		align-items: center;
		gap: 4px;

		.vote-btn {
			cursor: pointer;
			font-size: 18px;
			opacity: 0.6;
			border-radius: 999px;

			&:hover {
				opacity: 0.85;
			}

			&.active {
				opacity: 1;
			}
		}
	}
}
</style>
