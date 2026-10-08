<template>
<div class="admin-page admin-index-page page-width-xl flex-column-lg">

	<h2>Admin</h2>

	<p v-if="loginLoaded && !isAdmin" class="no-access"><em>This page is intended for admin use only.</em></p>

	<template v-else-if="isAdmin">
		<admin-section title="Users" v-model:open="open.users">
			<users-table v-if="visited.users"/>
		</admin-section>
		<admin-section title="Topics" v-model:open="open.topics">
			<topics-table v-if="visited.topics"/>
		</admin-section>
	</template>

</div>
</template>

<script>
import AdminSection from '@/pages/admin/admin-section.vue';
import UsersTable from '@/pages/admin/users-table.vue';
import TopicsTable from '@/pages/admin/topics-table.vue';

export default {
	components: {
		AdminSection,
		UsersTable,
		TopicsTable,
	},
	data() {
		return {
			open: {users: false, topics: false},
			visited: {users: false, topics: false},
		};
	},
	computed: {
		loginLoaded() {
			return this.$store.getters.loginLoaded;
		},
		isAdmin() {
			return this.$store.getters.isAdmin;
		},
	},
	watch: {
		// sections are loaded the first time they're opened and kept afterwards
		open: {
			handler(open) {
				Object.keys(open).forEach(name => {
					if (open[name]) {
						this.visited[name] = true;
					}
				});
			},
			deep: true,
		},
	},
};
</script>
