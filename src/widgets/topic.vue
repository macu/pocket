<template>
<div class="topic" :class="[sizeClass, {negative: negative, checkable: checkable, checked: checked, selected}]" @click="toggleCheck()">
	<material-icon
		v-if="checkable"
		class="topic-checkbox"
		:icon="checked ? 'check_box' : 'check_box_outline_blank'"
		:fill="checked"/>
	<el-tooltip v-if="authorUpvoted" content="Up-voted by the author" placement="top">
		<button v-if="nameSelectable" type="button" class="topic-name-button"
			aria-label="Toggle topic filter (up-voted by the author)" :aria-pressed="selected"
			@click.stop="$emit('select')">
			<material-icon class="topic-author" icon="signature"/>
		</button>
		<material-icon v-else class="topic-author" icon="signature"/>
	</el-tooltip>
	<button v-if="nameSelectable" type="button" class="topic-name topic-name-button"
		:aria-pressed="selected" @click.stop="$emit('select')"><slot/></button>
	<span v-else class="topic-name"><slot/></span>
	<span v-if="votable && authenticated" class="topic-votes" :class="{'is-pending': votePending}"
		:aria-busy="votePending" :aria-disabled="votePending" @click.stop>
		<material-icon
			v-if="!userVote || userVote === 'upvote'"
			icon="thumb_up" class="vote-btn upvote"
			:fill="userVote === 'upvote'"
			:class="{active: userVote === 'upvote'}"
			@click="setVote('upvote')"/>
		<material-icon
			v-if="!userVote || userVote === 'downvote'"
			icon="thumb_down" class="vote-btn downvote"
			:fill="userVote === 'downvote'"
			:class="{active: userVote === 'downvote'}"
			@click="setVote('downvote')"/>
	</span>
	<span v-if="showCount" class="topic-count">{{formattedCount}}</span>
</div>
</template>

<script>
import MaterialIcon from '@/widgets/material-icon.vue';

export default {
	components: {
		MaterialIcon,
	},
	emits: ['vote', 'check', 'select'],
	props: {
		nameSelectable: Boolean,
		selected: Boolean,
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
		countOutput: {
			type: String,
			default: '%d',
		},
		countOutputSingular: {
			type: String,
			default: '%d',
		},
		negative: {
			type: Boolean,
			default: false,
		},
		votable: {
			type: Boolean,
			default: false,
		},
		votePending: {
			type: Boolean,
			default: false,
		},
		authorUpvoted: {
			type: Boolean,
			default: false,
		},
		checkable: {
			type: Boolean,
			default: false,
		},
		checked: {
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
		formattedCount() {
			if (this.count === 1) {
				return this.countOutputSingular.replace('%d', this.count);
			}
			return this.countOutput.replace('%d', this.count);
		},
		authenticated() {
			return this.$store.getters.authenticated;
		},
	},
	methods: {
		setVote(vote) {
			if (!this.votePending) {
				this.$emit('vote', vote);
			}
		},
		undoVote() {
			this.$emit('vote', null);
		},
		toggleCheck() {
			if (!this.checkable) {
				return;
			}
			this.$emit('check', !this.checked);
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

	&.checkable {
		cursor: pointer;
	}

	&.checked,
	&.selected {
		outline: 2px solid white;
		outline-offset: 1px;
	}

	.topic-name-button {
		border: 0;
		padding: 0;
		background: transparent;
		color: inherit;
		font: inherit;
		text-align: inherit;
		cursor: pointer;

		&:hover {
			text-decoration: underline;
		}
	}

	.topic-author {
		font-size: 1.1em;
	}

	.topic-checkbox {
		font-size: 1.1em;
	}

	&.size-small {
		padding: 4px 8px;
		border-radius: 999px;
		font-size: 0.85em;
		gap: 6px;
	}

	&.size-medium {
		padding: 8px 10px;
		font-size: 0.95em;
		gap: 8px;
	}

	&.size-large {
		padding: 11px 14px;
		font-size: 1.15rem;
		gap: 10px;
	}

	.topic-name {
		display: inline-block;
		white-space: normal;
		word-break: break-word;
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
		&.is-pending .vote-btn {
			cursor: wait;
		}

		.vote-btn {
			cursor: pointer;
			font-size: 16px;
			padding: 2px;
			opacity: 0.6;
			border-radius: 999px;

			&:hover {
				opacity: 0.85;
			}

			&.active {
				opacity: 1;
				color: white;
				background-color: rgb(76, 175, 80);

				&.downvote {
					background-color: rgb(211, 86, 86);
				}
			}
		}
	}
}
</style>
