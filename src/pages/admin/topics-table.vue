<template>
<div class="admin-table-box">

	<div v-if="!userId" class="flex-column-sm admin-search">
		<small>Topic name</small>
		<el-input v-model="query" type="text" clearable
			@input="scheduleReload()"
			@keyup.enter="reloadNow()"/>
	</div>

	<div class="admin-toolbar">
		<span class="total-items">
			{{total}} {{total === 1 ? 'topic' : 'topics'}}<template v-if="selectedIds.length > 0">, {{selectedIds.length}} selected</template>
		</span>
	</div>

	<div class="admin-table-controls">
		<div class="topic-bulk-actions">
			<el-button @click="setSelectedBanned(true)" type="warning"
				:disabled="selectedIds.length === 0 || updating">
				Ban selected
			</el-button>
			<el-button @click="setSelectedBanned(false)"
				:disabled="selectedIds.length === 0 || updating">
				Enable selected
			</el-button>
		</div>
		<el-pagination v-if="total > pageSize" class="admin-pagination"
			:current-page="page" :page-size="pageSize" :total="total"
			layout="prev, pager, next" :disabled="loading"
			@current-change="goToPage"/>
	</div>

	<div v-if="topics.length > 0" class="admin-table-wrap">
		<table class="admin-table">
			<thead>
				<tr>
					<th class="check">
						<el-checkbox :model-value="allSelected" :indeterminate="someSelected"
							@change="toggleAll($event)"/>
					</th>
					<th>Topic</th>
					<th>Status</th>
					<th v-if="!userId">Created by</th>
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
					<td>
						<span class="topic-status" :class="topic.banned ? 'is-banned' : 'is-enabled'">
							{{topic.banned ? 'Banned' : 'Enabled'}}
						</span>
					</td>
					<td v-if="!userId">
						<router-link :to="{name: 'admin-user', params: {id: topic.createdBy}}">
							{{topic.createdByName}}
							<small v-if="topic.createdByHandle">@{{topic.createdByHandle}}</small>
						</router-link>
					</td>
					<td><moment :time="topic.createdAt" ago/></td>
					<td class="number">{{topic.postCount}}</td>
					<td class="number">
						<el-button @click="setTopicBanned(topic, !topic.banned)" text :disabled="updating">
							{{topic.banned ? 'Enable' : 'Ban'}}
						</el-button>
					</td>
				</tr>
			</tbody>
		</table>
	</div>

	<p v-else-if="!loading" class="no-items"><em>No topics found.</em></p>

	<div class="admin-table-controls">
		<div class="topic-bulk-actions">
			<el-button @click="setSelectedBanned(true)" type="warning"
				:disabled="selectedIds.length === 0 || updating">
				Ban selected
			</el-button>
			<el-button @click="setSelectedBanned(false)"
				:disabled="selectedIds.length === 0 || updating">
				Enable selected
			</el-button>
		</div>
		<el-pagination v-if="total > pageSize" class="admin-pagination"
			:current-page="page" :page-size="pageSize" :total="total"
			layout="prev, pager, next" :disabled="loading"
			@current-change="goToPage"/>
	</div>

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

const SEARCH_DEBOUNCE_MS = 300;

// Lists topics created by userId if given, otherwise all topics on the site.
export default {
	props: {
		userId: {
			type: Number,
			default: null,
		},
	},
	data() {
		return {
			query: '',
			topics: [],
			total: 0,
			pageSize: 25,
			page: 1,
			loading: false,
			updating: false,
			selectedIds: [],
			reloadTimeout: null,
			requestId: 0,
		};
	},
	computed: {
		pageCount() {
			return Math.max(1, Math.ceil(this.total / this.pageSize));
		},
		allSelected() {
			return this.topics.length > 0 && this.selectedIds.length === this.topics.length;
		},
		someSelected() {
			return this.selectedIds.length > 0 && !this.allSelected;
		},
	},
	mounted() {
		this.load();
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
			return this.load();
		},
		goToPage(page) {
			this.page = Math.min(Math.max(1, page), this.pageCount);
			this.load();
		},
		load() {
			const requestId = ++this.requestId;
			this.loading = true;
			this.selectedIds = [];
			const params = {offset: (this.page - 1) * this.pageSize};
			if (this.userId) {
				params.userId = this.userId;
			} else {
				params.query = this.query;
			}
			return ajaxGet(this.userId ? '/ajax/admin/user/topics' : '/ajax/admin/topics', params).then(response => {
				if (requestId !== this.requestId) {
					return;
				}
				this.topics = response.topics || [];
				this.total = response.total || 0;
				this.pageSize = response.pageSize || this.pageSize;
				// the last page may have emptied after the result set changes
				if (this.topics.length === 0 && this.total > 0 && this.page > this.pageCount) {
					this.page = this.pageCount;
					return this.load();
				}
			}).finally(() => {
				if (requestId === this.requestId) {
					this.loading = false;
				}
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
		setTopicBanned(topic, banned) {
			this.setTopicsBanned([topic.id], banned);
		},
		setSelectedBanned(banned) {
			this.setTopicsBanned(this.selectedIds, banned);
		},
		setTopicsBanned(ids, banned) {
			this.updating = true;
			const params = {
				topicIds: ids.join(','),
				banned,
			};
			ajaxPost('/ajax/admin/topics/status', params).then(() => {
				const noun = ids.length === 1 ? 'Topic' : 'Topics';
				alertSuccess(`${noun} ${banned ? 'banned' : 'enabled'}.`);
				return this.load();
			}).finally(() => {
				this.updating = false;
			});
		},
	},
};
</script>

<style lang="scss">
.admin-search {
	max-width: 320px;
}

.topic-status {
	&.is-banned { color: rgb(240, 100, 100); }
	&.is-enabled { color: rgb(100, 210, 130); }
}

.topic-bulk-actions {
	display: flex;
	align-items: center;
	gap: 8px;
}

.admin-table-controls {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 12px;

	.admin-pagination {
		flex: 0 0 auto;
		justify-content: flex-end;
		margin: 0;
	}
}

@media (max-width: 600px) {
	.admin-table-controls {
		align-items: flex-start;
		flex-direction: column;
		.admin-pagination {
			justify-content: flex-start;
		}
	}
}
</style>
