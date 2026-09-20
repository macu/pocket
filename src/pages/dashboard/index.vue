<template>
<div class="dashboard-page flex-column-lg page-width-md">

	<return-to-top/>

	<horizontal-controls v-if="loginLoaded">

		<el-button @click="addTopic()" type="primary">
			Add Topic
		</el-button>

		<el-button @click="createPost()" type="primary">
			Create Post
		</el-button>

	</horizontal-controls>

	<div class="flex-column-lg">

		<template v-if="showingAddTopic">
			<div class="add-topic-form">
				<form-field title="Topic name">
					<el-input v-model="newTopicName" type="text" maxlength="50"
						autocapitalize="words"
						@keyup.enter.native="submitAddTopic()"
					/>
				</form-field>

				<div class="add-topic-actions">
					<el-button @click="submitAddTopic()" :disabled="addTopicDisabled" type="primary">
						Save Topic
					</el-button>
					<el-button @click="cancelAddTopic()">Cancel</el-button>
				</div>
			</div>
		</template>

		<loading-message v-else-if="loading"/>

		<div v-else class="flex-column">

			<h2>Top Topics</h2>

			<div v-if="topTopics.length > 0" class="top-topics flex-row-lg">
				<div v-for="topic in topTopics" :key="topic.id" class="top-topic flex-row" :class="{'negative': topic.sum < 0}">
					<div>{{topic.name}}</div>
					<div class="count">{{topic.sum}}</div>
				</div>
			</div>

			<p v-else>No topics available.</p>

		</div>

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
	showError,
} from '@/utils/notify.js';

export default {
	components: {
	},
	data() {
		return {
			loading: true,
			topTopics: [],
			lastTopicsLength: 0,

			showingAddTopic: false,
			newTopicName: '',
			addTopicLoading: false,
		};
	},
	computed: {
		loginLoaded() {
			return this.$store.getters.loginLoaded;
		},
		showLoadMoreTopics() {
			return this.lastTopicsLength > 0;
		},
		addTopicDisabled() {
			return this.addTopicLoading || !this.newTopicName.trim();
		},
		cancelAddTopicDisabled() {
			return this.addTopicLoading;
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
		addTopic() {
			this.showingAddTopic = true;
			this.newTopicName = '';
		},
		cancelAddTopic() {
			this.showingAddTopic = false;
			this.newTopicName = '';
			this.addTopicLoading = false;
		},
		submitAddTopic() {
			if (this.addTopicDisabled) {
				return;
			}
			this.addTopicLoading = true;
			ajaxPost('/ajax/topic', {
				name: this.newTopicName,
			}, {
				// special error codes
				'invalid-topic-name': 'The topic name provided is invalid.',
			}).then(response => {
				if (response.exists) {
					showError('That topic already exists.');
					return;
				}
				if (response.topic) {
					this.topTopics.unshift(response.topic);
				}
				this.showingAddTopic = false;
				this.newTopicName = '';
				alertSuccess('Topic added.');
			}).finally(() => {
				this.addTopicLoading = false;
			});
		},
		loadDashboard() {
			this.loading = true;
			ajaxGet('/ajax/dashboard').then(response => {
				this.topTopics = response.topTopics || [];
				this.lastTopicsLength = this.topTopics.length;
			}).finally(() => {
				this.loading = false;
			});
		},
		createPost() {
			// TODO
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

	.add-topic-form {
		padding: 12px;
		border-radius: $border-radius;
		background-color: rgba(255, 255, 255, 0.04);
	}

	.add-topic-actions {
		display: flex;
		gap: 8px;
		margin-top: 12px;
	}

	.top-topics {
		margin-top: 20px;
		.top-topic {
			padding: 10px;
			border-radius: $border-radius;
			background-color: rgb(86, 86, 211);
			color: white;
			&.negative {
				background-color: rgb(211, 86, 86);
			}
			>.count {
				font-weight: bold;
				background-color: rgba(255, 255, 255, 0.1);
				padding: 2px 6px;
				border-radius: $border-radius;
			}
		}
	}
}
</style>
