<template>
<el-dialog :model-value="modelValue" @update:model-value="$emit('update:modelValue', $event)"
	@open="load()" title="Revisions" width="90%" class="post-revisions-dialog" align-center append-to-body>

	<loading-message v-if="loading"/>

	<div v-else class="flex-column-md">
		<div v-for="(revision, index) in visibleRevisions" :key="index" class="revision flex-column-sm"
			:class="{current: revision.current}">
			<small>{{revision.current ? 'Current revision' : 'Revision'}} &middot; <moment :time="revision.createdAt" ago/></small>
			<div class="revision-diff">
				<template v-if="index < revisions.length - 1">
					<template v-for="(part, partIndex) in diffParts(index)" :key="partIndex">
						<ins v-if="part.added">{{part.value}}</ins>
						<del v-else-if="part.removed">{{part.value}}</del>
						<span v-else>{{part.value}}</span>
					</template>
				</template>
				<template v-else>{{revision.postText}}</template>
			</div>
		</div>
	</div>

	<template #footer>
		<el-pagination v-if="total > pageSize" class="revision-pagination"
			:current-page="page" :page-size="pageSize" :total="total"
			layout="prev, pager, next" :disabled="loading"
			@current-change="load"/>
		<el-button @click="$emit('update:modelValue', false)">Close</el-button>
	</template>
</el-dialog>
</template>

<script>
import {diffWords} from 'diff';

import {
	ajaxGet,
} from '@/utils/ajax.js';

export default {
	props: {
		modelValue: Boolean,
		postId: {
			type: [String, Number],
			required: true,
		},
	},
	emits: ['update:modelValue'],
	data() {
		return {
			revisions: [],
			total: 0,
			pageSize: 20,
			page: 1,
			loading: false,
			requestId: 0,
		};
	},
	computed: {
		visibleRevisions() {
			return this.revisions.slice(0, this.pageSize);
		},
	},
	methods: {
		load(page = 1) {
			this.page = page;
			this.loading = true;
			const requestId = ++this.requestId;
			ajaxGet('/ajax/post/revisions', {
				id: this.postId,
				offset: (page - 1) * this.pageSize,
			}).then(response => {
				if (requestId !== this.requestId) {
					return;
				}
				this.revisions = response.revisions || [];
				this.total = response.total || 0;
				this.pageSize = response.pageSize || this.pageSize;
			}).finally(() => {
				if (requestId === this.requestId) {
					this.loading = false;
				}
			});
		},
		// diff of each revision against the one before it (revisions are newest first)
		diffParts(index) {
			return diffWords(this.revisions[index + 1].postText, this.revisions[index].postText);
		},
	},
};
</script>

<style lang="scss">
@import '@/styles/vars.scss';

.post-revisions-dialog.el-dialog {
	max-width: 800px;
	padding: 0;
	border-radius: $border-radius;
	overflow: hidden;
	background: $app-bg-color;
	color: $app-fg-color;
	border: 1px solid $app-separator-color;

	.el-dialog__header {
		margin: 0;
		padding: 14px 20px;
		background: rgba(255, 255, 255, 0.08);
		border-bottom: 1px solid $app-separator-color;
	}
	.el-dialog__title {
		color: $app-fg-color;
		font-weight: 600;
	}
	.el-dialog__headerbtn .el-dialog__close {
		color: $app-fg-color;
	}
	.el-dialog__body {
		padding: 16px 20px;
		max-height: 60vh;
		overflow-y: auto;
		color: $app-fg-color;
	}
	.el-dialog__footer {
		padding: 12px 20px;
		background: rgba(255, 255, 255, 0.08);
		border-top: 1px solid $app-separator-color;
		text-align: right;
	}

	.revision {
		padding: 12px;
		border: 1px solid rgba(255, 255, 255, 0.2);
		border-radius: $border-radius;
		background: rgba(255, 255, 255, 0.04);

		small {
			opacity: 0.7;
			font-weight: 600;
		}

		&.current {
			border-color: $topic-bg-color;
			background: rgba(86, 86, 211, 0.15);
		}
	}
	.revision-diff {
		white-space: pre-wrap;
		word-break: break-word;
		line-height: 1.5;
	}
	ins {
		background: rgb(150, 235, 160);
		color: black;
		text-decoration: none;
		border-radius: 2px;
	}
	del {
		background: rgb(255, 170, 170);
		color: black;
		border-radius: 2px;
	}
}
</style>
