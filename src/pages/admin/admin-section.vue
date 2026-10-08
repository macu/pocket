<template>
<section class="admin-section">
	<h3 class="admin-section-header" @click="$emit('update:open', !open)">
		<material-icon :icon="open ? 'expand_more' : 'chevron_right'"/>
		<span>{{title}}</span>
	</h3>
	<div v-if="open" class="admin-section-content">
		<slot/>
	</div>
</section>
</template>

<script>
export default {
	props: {
		title: {
			type: String,
			required: true,
		},
		open: {
			type: Boolean,
			default: false,
		},
	},
	emits: ['update:open'],
};
</script>
