<template>
<div class="topic" :class="[sizeClass, {negative: negative}]">
	<span class="topic-name"><slot/></span>
	<span v-if="showCount" class="topic-count">{{count}}</span>
</div>
</template>

<script>
export default {
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
	},
	computed: {
		sizeClass() {
			return 'size-' + this.size;
		},
		showCount() {
			return this.count !== null && this.count !== undefined;
		}
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
}
</style>
