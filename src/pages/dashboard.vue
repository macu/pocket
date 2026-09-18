<template>
<div class="dashboard-page flex-column-lg page-width-md">

	<return-to-top/>

	<horizontal-controls>

	</horizontal-controls>

	<div class="flex-column-lg">

		<p>Coming soon</p>

	</div>

</div>
</template>

<script>
export default {
	components: {
	},
	data() {
		return {
		};
	},
	computed: {
		authenticated() {
			return this.$store.getters.authenticated;
		},
	},
	watch: {
		filter: {
			handler() {
				this.loadDashboard();
			},
			deep: true,
		},
	},
	mounted() {
		this.loadDashboard();
	},
	methods: {
		loadDashboard() {
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
}
</style>
