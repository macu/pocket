<template>
<div class="admin-user-page" :class="isMobile ? 'page-width-md' : 'page-width-xl'">

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

				<h2>Topics created</h2>

				<div class="admin-box admin-topics-box flex-column-sm">

					<div class="admin-topics-toolbar">
						<span class="total-topics">
							{{total}} {{total === 1 ? 'topic' : 'topics'}}<template v-if="selectedIds.length > 0">, {{selectedIds.length}} selected</template>
						</span>
						<el-button @click="deleteSelected()" type="danger" size="small"
							:disabled="selectedIds.length === 0 || bulkDeleting">
							Delete selected
						</el-button>
					</div>

					<horizontal-controls v-if="total > pageSize" class="admin-pagination">
						<el-button @click="goToPage(page - 1)" :disabled="topicsLoading || page <= 1" size="small">Previous</el-button>
						<span>Page {{page}} of {{pageCount}}</span>
						<el-button @click="goToPage(page + 1)" :disabled="topicsLoading || page >= pageCount" size="small">Next</el-button>
					</horizontal-controls>

					<div v-if="topics.length > 0" class="admin-topics-table-wrap">
						<table class="admin-topics-table">
							<thead>
								<tr>
									<th class="check">
										<el-checkbox :model-value="allSelected" :indeterminate="someSelected"
											@change="toggleAll($event)"/>
									</th>
									<th>Topic</th>
									<th>Created</th>
									<th class="number">Posts</th>
									<th></th>
								</tr>
							</thead>
							<tbody>
								<tr v-for="topic in topics" :key="topic.id">
									<td class="check">
										<el-checkbox :model-value="selectedIds.includes(topic.id)"
											@change="toggleOne(topic.id, $event)"/>
									</td>
									<td>{{topic.name}}</td>
									<td><moment :time="topic.createdAt" ago/></td>
									<td class="number">{{topic.postCount}}</td>
									<td class="number">
										<el-button @click="deleteTopic(topic)" type="danger" size="small" text
											:disabled="bulkDeleting">
											Delete
										</el-button>
									</td>
								</tr>
							</tbody>
						</table>
					</div>

					<p v-else-if="!topicsLoading" class="no-topics"><em>No topics created.</em></p>

					<horizontal-controls v-if="total > pageSize" class="admin-pagination">
						<el-button @click="goToPage(page - 1)" :disabled="topicsLoading || page <= 1" size="small">Previous</el-button>
						<span>Page {{page}} of {{pageCount}}</span>
						<el-button @click="goToPage(page + 1)" :disabled="topicsLoading || page >= pageCount" size="small">Next</el-button>
					</horizontal-controls>

				</div>

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

import {
	USER_AVAILABLE_ROLES,
} from '@/const.js';

export default {
	data() {
		return {
			loading: true,
			user: null,
			assignableStatuses: USER_AVAILABLE_ROLES,
			statusSaving: false,
			deleting: false,
			topics: [],
			total: 0,
			pageSize: 25,
			page: 1,
			topicsLoading: false,
			selectedIds: [],
			bulkDeleting: false,
			topicsRequestId: 0,
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
		allSelected() {
			return this.topics.length > 0 && this.selectedIds.length === this.topics.length;
		},
		someSelected() {
			return this.selectedIds.length > 0 && !this.allSelected;
		},
		userId() {
			return this.$route.params.id;
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
		userId() {
			if (this.isAdmin) {
				this.load();
			}
		},
	},
	methods: {
		load() {
			this.loading = true;
			this.page = 1;
			this.topics = [];
			this.total = 0;
			ajaxGet('/ajax/admin/user', {
				userId: this.userId,
			}).then(response => {
				this.user = response.user || null;
				if (this.user) {
					this.loadTopics();
				}
			}).catch(() => {
				this.user = null;
			}).finally(() => {
				this.loading = false;
			});
		},
		loadTopics() {
			const requestId = ++this.topicsRequestId;
			this.topicsLoading = true;
			this.selectedIds = [];
			return ajaxGet('/ajax/admin/user/topics', {
				userId: this.user.id,
				offset: (this.page - 1) * this.pageSize,
			}).then(response => {
				if (requestId !== this.topicsRequestId) {
					return;
				}
				this.topics = response.topics || [];
				this.total = response.total || 0;
				this.pageSize = response.pageSize || this.pageSize;
				// the last page may have emptied after deletions
				if (this.topics.length === 0 && this.total > 0 && this.page > this.pageCount) {
					this.page = this.pageCount;
					return this.loadTopics();
				}
			}).finally(() => {
				if (requestId === this.topicsRequestId) {
					this.topicsLoading = false;
				}
			});
		},
		goToPage(page) {
			this.page = Math.min(Math.max(1, page), this.pageCount);
			this.loadTopics();
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
		toggleAll(checked) {
			this.selectedIds = checked ? this.topics.map(topic => topic.id) : [];
		},
		toggleOne(id, checked) {
			this.selectedIds = checked ?
				[...this.selectedIds, id] :
				this.selectedIds.filter(selectedId => selectedId !== id);
		},
		deleteTopic(topic) {
			this.confirmDeleteTopics([topic.id], `the topic "${topic.name}"`, topic.postCount);
		},
		deleteSelected() {
			const selected = this.topics.filter(topic => this.selectedIds.includes(topic.id));
			const postCount = selected.reduce((sum, topic) => sum + topic.postCount, 0);
			this.confirmDeleteTopics(selected.map(topic => topic.id),
				`${selected.length} ${selected.length === 1 ? 'topic' : 'topics'}`, postCount);
		},
		confirmDeleteTopics(ids, description, postCount) {
			this.$confirm(`Delete ${description}? ${postCount > 0 ? `${ids.length === 1 ? 'It is' : 'They are'} used on ${postCount} posts and will be removed. ` : ''}This cannot be undone.`, 'Delete topics', {
				confirmButtonText: 'Delete',
				cancelButtonText: 'Cancel',
				type: 'warning',
			}).then(() => {
				this.bulkDeleting = true;
				ajaxPost('/ajax/admin/user/topics/delete', {
					userId: this.user.id,
					topicIds: ids.join(','),
				}).then(() => {
					alertSuccess(ids.length === 1 ? 'Topic deleted.' : 'Topics deleted.');
					return this.loadTopics();
				}).finally(() => {
					this.bulkDeleting = false;
				});
			}).catch(() => {
				// User cancelled
			});
		},
	},
};
</script>

<style lang="scss">
@import '@/styles/vars.scss';

.admin-user-page {
	color: $app-fg-color;

	.no-access, .no-topics, .total-topics {
		opacity: 0.7;
		font-size: 0.9em;
	}

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

			>.admin-user-column-left {
				flex-basis: auto;
				width: 100%;
			}

			>.admin-user-column-right {
				width: 100%;
			}
		}
	}

	.admin-user-header {
		.handle {
			opacity: 0.7;
		}
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

	.admin-box {
		border: 1px solid rgba(255, 255, 255, 0.2);
		border-radius: 6px;
		padding: 12px;
		align-items: flex-start;

		h3 {
			margin: 0;
		}
	}

	.admin-topics-box {
		align-items: stretch;
	}

	.admin-topics-toolbar {
		display: flex;
		align-items: center;
		justify-content: space-between;
		column-gap: 12px;
	}

	.admin-pagination {
		justify-content: center;
		align-items: center;
		margin: 8px 0;
	}

	.admin-topics-table-wrap {
		overflow-x: auto;
	}

	.admin-topics-table {
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
			border-bottom: 3px solid rgba(255, 255, 255, 0.35);
		}

		.check {
			width: 1%;
		}

		.number {
			text-align: right;
		}

		tbody tr:hover {
			background: rgba(255, 255, 255, 0.05);
		}
	}
}
</style>
