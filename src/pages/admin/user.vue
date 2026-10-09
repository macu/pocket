<template>
<div class="admin-page admin-user-page" :class="isMobile ? 'page-width-md' : 'page-width-xl'">

	<p v-if="loginLoaded && !isAdmin" class="no-access"><em>This page is intended for admin use only.</em></p>

	<template v-else-if="isAdmin">

		<loading-message v-if="loading"/>

		<p v-else-if="!user" class="no-access"><em>User not found.</em></p>

		<div v-else class="admin-user-columns" :class="{'is-mobile': isMobile}">

			<div class="admin-user-column-left flex-column-lg">

				<div class="admin-user-header flex-column-sm">
					<h2>{{user.displayName}}</h2>
					<small v-if="user.handle" class="handle">@{{user.handle}}</small>
					<dl class="admin-user-details">
						<dt>Email</dt>
						<dd>{{user.email}}</dd>
						<dt>Joined</dt>
						<dd><moment :time="user.createdAt" ago/></dd>
						<dt>Posts</dt>
						<dd>{{user.posts}}</dd>
						<dt>Post votes</dt>
						<dd>{{user.postVotes}}</dd>
						<dt>Topic votes</dt>
						<dd>{{user.topicVotes}}</dd>
					</dl>
					<router-link :to="{name: 'user', params: {identifier: user.handle || String(user.id)}}">
						View public page
					</router-link>
				</div>

				<div class="admin-box flex-column-sm">
					<h3>Status</h3>
					<el-select :model-value="user.role" :disabled="user.role === 'admin' || statusSaving"
						@change="changeStatus($event)">
						<el-option v-if="user.role === 'admin'" value="admin" label="admin"/>
						<el-option v-for="status in assignableStatuses" :key="status" :value="status" :label="status"/>
					</el-select>
				</div>

				<div v-if="user.role !== 'admin'" class="admin-box flex-column-sm">
					<h3>Delete content</h3>
					<small>Removes all posts and votes by this user. Topics they created are kept.</small>
					<el-button @click="deleteContent()" type="danger" :disabled="deleting">
						Delete all posts and votes
					</el-button>
				</div>

			</div>

			<div class="admin-user-column-right flex-column-lg">

				<admin-section title="Topics created" v-model:open="open.topics">
					<topics-table v-if="visited.topics" :user-id="user.id"/>
				</admin-section>
				<admin-section title="Posts" v-model:open="open.posts">
					<user-posts-table v-if="visited.posts" :user-id="user.id"/>
				</admin-section>

			</div>

		</div>

	</template>

</div>
</template>

<script>
import {
	ajaxGet,
	ajaxPost,
} from '@/utils/ajax.js';

import {
	alertSuccess,
} from '@/utils/notify.js';

import AdminSection from '@/pages/admin/admin-section.vue';
import TopicsTable from '@/pages/admin/topics-table.vue';
import UserPostsTable from '@/pages/admin/user-posts-table.vue';

import {
	USER_AVAILABLE_ROLES,
} from '@/const.js';

export default {
	components: {
		AdminSection,
		TopicsTable,
		UserPostsTable,
	},
	data() {
		return {
			loading: true,
			user: null,
			assignableStatuses: USER_AVAILABLE_ROLES,
			statusSaving: false,
			deleting: false,
			open: {topics: false, posts: false},
			visited: {topics: false, posts: false},
		};
	},
	computed: {
		loginLoaded() {
			return this.$store.getters.loginLoaded;
		},
		isAdmin() {
			return this.$store.getters.isAdmin;
		},
		isMobile() {
			return this.$store.getters.isMobile;
		},
		userId() {
			return this.$route.params.id;
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
		userId() {
			if (this.isAdmin) {
				this.load();
			}
		},
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
	methods: {
		refreshUser() {
			return ajaxGet('/ajax/admin/user', {
				userId: this.userId,
			}).then(response => {
				this.user = response.user || this.user;
			});
		},
		load() {
			this.loading = true;
			ajaxGet('/ajax/admin/user', {
				userId: this.userId,
			}).then(response => {
				this.user = response.user || null;
			}).catch(() => {
				this.user = null;
			}).finally(() => {
				this.loading = false;
			});
		},
		changeStatus(status) {
			this.$confirm(`Change the status of ${this.user.displayName} to "${status}"?`, 'Change status', {
				confirmButtonText: 'Change',
				cancelButtonText: 'Cancel',
				type: 'warning',
			}).then(() => {
				this.statusSaving = true;
				ajaxPost('/ajax/admin/user/status', {
					userId: this.user.id,
					status,
				}, {
					'user-is-admin': 'The status of an admin cannot be changed.',
				}).then(() => {
					this.user.role = status;
					alertSuccess('Status updated.');
				}).finally(() => {
					this.statusSaving = false;
				});
			}).catch(() => {
				// User cancelled
			});
		},
		deleteContent() {
			this.$confirm(`Delete all posts and votes by ${this.user.displayName}? Topics they created are kept. This cannot be undone.`, 'Delete all posts and votes', {
				confirmButtonText: 'Delete all',
				cancelButtonText: 'Cancel',
				type: 'warning',
			}).then(() => {
				this.deleting = true;
				ajaxPost('/ajax/admin/user/delete-content', {
					userId: this.user.id,
				}, {
					'user-is-admin': 'The content of an admin cannot be deleted.',
				}).then(() => {
					alertSuccess('User content deleted.');
					this.load();
				}).finally(() => {
					this.deleting = false;
				});
			}).catch(() => {
				// User cancelled
			});
		},
	},
};
</script>

<style lang="scss">
.admin-user-page {

	.admin-user-columns {
		display: flex;
		align-items: flex-start;
		column-gap: 40px;

		>.admin-user-column-left {
			flex: 0 0 300px;
		}

		>.admin-user-column-right {
			flex: 1;
			min-width: 0;
		}

		&.is-mobile {
			flex-direction: column;
			row-gap: 40px;
			width: 100%;

			>.admin-user-column-left,
			>.admin-user-column-right {
				flex-basis: auto;
				width: 100%;
			}
		}
	}

	.admin-user-header .handle {
		opacity: 0.7;
	}

	.admin-user-details {
		display: grid;
		grid-template-columns: auto 1fr;
		gap: 4px 16px;
		margin: 8px 0;

		dt {
			opacity: 0.7;
			font-size: 0.9em;
		}

		dd {
			margin: 0;
			word-break: break-all;
		}
	}
}
</style>
