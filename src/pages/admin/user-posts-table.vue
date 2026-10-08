<template>
<div class="admin-table-box">

	<div class="admin-toolbar">
		<span class="total-items">{{total}} {{total === 1 ? 'post' : 'posts'}}</span>
	</div>

	<horizontal-controls v-if="total > pageSize" class="admin-pagination">
		<el-button @click="goToPage(page - 1)" :disabled="loading || page <= 1">Previous</el-button>
		<span>Page {{page}} of {{pageCount}}</span>
		<el-button @click="goToPage(page + 1)" :disabled="loading || page >= pageCount">Next</el-button>
	</horizontal-controls>

	<div v-if="posts.length > 0" class="admin-table-wrap">
		<table class="admin-table">
			<thead>
				<tr>
					<th>Post</th>
					<th>Created</th>
					<th class="number">Up</th>
					<th class="number">Down</th>
					<th class="number">Sum</th>
					<th class="number">Sub-posts</th>
				</tr>
			</thead>
			<tbody>
				<tr v-for="post in posts" :key="post.id">
					<td class="wrap">
						<router-link :to="{name: 'post', params: {id: post.id}}">
							{{post.postText}}
						</router-link>
						<small v-if="post.parentPostId"> (reply)</small>
					</td>
					<td><moment :time="post.createdAt" ago/></td>
					<td class="number">{{post.upvotes}}</td>
					<td class="number">{{post.downvotes}}</td>
					<td class="number">{{post.sum}}</td>
					<td class="number">{{post.subPosts}}</td>
				</tr>
			</tbody>
		</table>
	</div>

	<p v-else-if="!loading" class="no-items"><em>No posts found.</em></p>

	<horizontal-controls v-if="total > pageSize" class="admin-pagination">
		<el-button @click="goToPage(page - 1)" :disabled="loading || page <= 1">Previous</el-button>
		<span>Page {{page}} of {{pageCount}}</span>
		<el-button @click="goToPage(page + 1)" :disabled="loading || page >= pageCount">Next</el-button>
	</horizontal-controls>

</div>
</template>

<script>
import {
	ajaxGet,
} from '@/utils/ajax.js';

export default {
	props: {
		userId: {
			type: Number,
			required: true,
		},
	},
	data() {
		return {
			posts: [],
			total: 0,
			pageSize: 25,
			page: 1,
			loading: false,
			requestId: 0,
		};
	},
	computed: {
		pageCount() {
			return Math.max(1, Math.ceil(this.total / this.pageSize));
		},
	},
	mounted() {
		this.load();
	},
	methods: {
		goToPage(page) {
			this.page = Math.min(Math.max(1, page), this.pageCount);
			this.load();
		},
		load() {
			const requestId = ++this.requestId;
			this.loading = true;
			return ajaxGet('/ajax/admin/user/posts', {
				userId: this.userId,
				offset: (this.page - 1) * this.pageSize,
			}).then(response => {
				if (requestId !== this.requestId) {
					return;
				}
				this.posts = response.posts || [];
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
