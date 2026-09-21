<template>
<span class="material-symbols-outlined" :class="classes">{{icon}}</span>
</template>

<script>
export default {
	props: {
		icon: {
			type: String,
			required: true,
		},
		large: {
			type: Boolean,
			default: false,
		},
		fill: {
			type: Boolean,
			default: false,
		},
	},
	computed: {
		classes() {
			return {
				large: this.large,
				fill: !!this.fill,
			}
		},
	},
};
</script>

<style lang="scss">
.material-symbols-outlined {
	font-family: 'Material Symbols Outlined';
	font-size: 24px;
	&.large {
		font-size: 48px;
	}
	&.fill {
		font-variation-settings: 'FILL' 1, 'wght' 400, 'GRAD' 0, 'opsz' 24;
	}
}
</style>
