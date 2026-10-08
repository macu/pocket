<template>
<div class="admin-users-page page-width-xl flex-column-lg">

	<h2>Users</h2>

	<p v-if="loginLoaded && !isAdmin" class="no-access"><em>This page is intended for admin use only.</em></p>

	<template v-else-if="isAdmin">

		<horizontal-controls class="filters align-start">
			<div class="flex-column-sm admin-users-search">
				<small>Name, handle or email</small>
				<el-input v-model="query" type="text" clearable
					@input="scheduleReload()"
					@keyup.enter="reloadNow()"/>
			</div>
			<div class="flex-column-sm">
				<small>Status</small>
				<el-select v-model="status" class="admin-users-status" @change="reloadNow()">
					<el-option value="" label="Any status"/>
					<el-option v-for="s in statuses" :key="s" :value="s" :label="s"/>
				</el-select>
			</div>
		</horizontal-controls>

		<p class="total-users">{{total}} {{total === 1 ? 'user' : 'users'}}<template v-if="filtered"> matching</template></p>

		<loading-message v-if="loading"/>

		<div v-else-if="users.length > 0" class="admin-users-table-wrap">
			<table class="admin-users-table">
				<thead>
					<tr>
						<th>Name</th>
						<th>Email</th>
						<th>Status</th>
						<th class="number">Topic votes</th>
						<th class="number">Post votes</th>
						<th class="number">Posts</th>
					</tr>
				</thead>
				<tbody>
					<tr v-for="user in users" :key="user.id">
						<td>
							<router-link :to="{name: 'admin-user', params: {id: user.id}}">
								{{user.displayName}}
								<small v-if="user.handle">@{{user.handle}}</small>
							</router-link>
						</td>
						<td>{{user.email}}</td>
						<td><span class="status" :class="'status-' + user.role">{{user.role}}</span></td>
						<td class="number">{{user.topicVotes}}</td>
						<td class="number">{{user.postVotes}}</td>
						<td class="number">{{user.posts}}</td>
					</tr>
				</tbody>
			</table>
		</div>

		<p v-else class="no-users"><em>No users found.</em></p>

		<horizontal-controls v-if="total > pageSize">
			<el-button @click="goToPage(page - 1)" :disabled="loading || page <= 1">Previous</el-button>
			<span>Page {{page}} of {{pageCount}}</span>
			<el-button @click="goToPage(page + 1)" :disabled="loading || page >= pageCount">Next</el-button>
		</horizontal-controls>

	</template>

</div>
</template>

<script>
import {
	ajaxGet,
} from '@/utils/ajax.js';

const SEARCH_DEBOUNCE_MS = 300;

export default {
	data() {
		return {
			query: '',
			status: '',
			statuses: ['admin', 'moderator', 'user', 'inactive', 'banned'],
			users: [],
			total: 0,
			pageSize: 25,
			page: 1,
			loading: false,
			reloadTimeout: null,
			requestId: 0,
		};
	},
	computed: {
		loginLoaded() {
			return this.$store.getters.loginLoaded;
		},
		isAdmin() {
			return this.$store.getters.isAdmin;
		},
		filtered() {
			return !!this.query.trim() || !!this.status;
		},
		pageCount() {
			return Math.max(1, Math.ceil(this.total / this.pageSize));
		},
	},
	watch: {
		isAdmin: {
			handler(isAdmin) {
				if (isAdmin) {
					this.load();
				}
			},
			immediate: true,
		},
	},
	beforeUnmount() {
		clearTimeout(this.reloadTimeout);
	},
	methods: {
		scheduleReload() {
			clearTimeout(this.reloadTimeout);
			this.reloadTimeout = setTimeout(() => this.reloadNow(), SEARCH_DEBOUNCE_MS);
		},
		reloadNow() {
			clearTimeout(this.reloadTimeout);
			this.page = 1;
			this.load();
		},
		goToPage(page) {
			this.page = Math.min(Math.max(1, page), this.pageCount);
			this.load();
		},
		load() {
			const requestId = ++this.requestId;
			this.loading = true;
			ajaxGet('/ajax/admin/users', {
				query: this.query,
				status: this.status,
				offset: (this.page - 1) * this.pageSize,
			}).then(response => {
				if (requestId !== this.requestId) {
					return;
				}
				this.users = response.users || [];
				this.total = response.total || 0;
				this.pageSize = response.pageSize || this.pageSize;
			}).finally(() => {
				if (requestId === this.requestId) {
					this.loading = false;
				}
			});
		},
	},
};
</script>

<style lang="scss">
.admin-users-page {

	.admin-users-status {
		min-width: 160px;
	}

	.admin-users-search {
		min-width: 260px;
	}

	.total-users, .no-users, .no-access {
		opacity: 0.7;
		font-size: 0.9em;
	}

	.admin-users-table-wrap {
		overflow-x: auto;
	}

	.admin-users-table {
		width: 100%;
		border-collapse: collapse;

		th, td {
			padding: 8px 12px;
			text-align: left;
			border-bottom: 1px solid rgba(255, 255, 255, 0.15);
			white-space: nowrap;
		}

		th {
			background: rgba(255, 255, 255, 0.08);
		}

		.number {
			text-align: right;
		}

		tbody tr:hover {
			background: rgba(255, 255, 255, 0.05);
		}

		small {
			opacity: 0.7;
		}
	}

	.status {
		padding: 2px 8px;
		border-radius: 4px;
		background: rgba(255, 255, 255, 0.1);

		&.status-moderator { background: rgba(86, 86, 211, 0.4); }
		&.status-admin { background: rgba(60, 160, 90, 0.4); }
		&.status-banned { background: rgba(200, 60, 60, 0.4); }
		&.status-inactive { opacity: 0.6; }
	}
}
</style>
