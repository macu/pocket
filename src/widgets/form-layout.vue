<template>
<div class="form-layout flex-column-lg">
	<h2 v-if="showTitle"><slot name="title">{{title}}</slot></h2>
	<slot/>
</div>
</template>

<script>
export default {
	props: {
		title: {
			type: String,
			required: false,
		},
	},
	computed: {
		showTitle() {
			return !!this.title || this.$slots.title !== undefined;
		},
	},
};
</script>

<style lang="scss">
@import '@/styles/vars.scss';

.form-layout {
	align-items: stretch;
	border: thin solid white;
	padding: 12px;
	border-radius: $border-radius;
}
</style>
