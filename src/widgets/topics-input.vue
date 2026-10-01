<template>
<div class="topics-input flex-column-sm">
	<topic-search ref="search"
		v-model="pendingValue"
		:disabled="atMax"
		:placeholder="atMax ? 'Max topics reached' : 'Add a topic'"
		@select="selectSuggestion"
		@submit="addPending()">
			<template #append>
				<el-button @click="submitPending()" type="primary" :disabled="addDisabled">Add</el-button>
			</template>
	</topic-search>
	<div v-if="modelValue.length" class="topics-input-tags flex-row-sm">
		<span v-for="(name, index) in modelValue" :key="name" class="topics-input-tag">
			{{name}}
			<material-icon icon="close" class="remove-btn" @click.stop="removeAt(index)"/>
		</span>
	</div>
</div>
</template>

<script>
import TopicSearch from '@/widgets/topic-search.vue';

export default {
	components: {
		TopicSearch,
	},
	props: {
		modelValue: {
			type: Array,
			default: () => [],
		},
		max: {
			type: Number,
			default: null,
		},
	},
	emits: ['update:modelValue'],
	data() {
		return {
			pendingValue: '',
		};
	},
	computed: {
		atMax() {
			return this.max !== null && this.modelValue.length >= this.max;
		},
		addDisabled() {
			return this.atMax || !this.pendingValue.trim();
		},
	},
	methods: {
		focus() {
			if (this.$refs.search && this.$refs.search.focus) {
				this.$refs.search.focus();
			}
		},
		submitPending() {
			if (this.$refs.search && this.$refs.search.submit) {
				this.$refs.search.submit();
			}
		},
		addPending() {
			this.addName(this.pendingValue);
		},
		selectSuggestion(topic) {
			this.addName(topic.name);
		},
		addName(rawName) {
			if (this.atMax) {
				return;
			}
			const name = rawName.trim();
			if (!name || this.modelValue.includes(name)) {
				this.pendingValue = '';
				return;
			}
			this.$emit('update:modelValue', [...this.modelValue, name]);
			this.pendingValue = '';
		},
		removeAt(index) {
			const updated = [...this.modelValue];
			updated.splice(index, 1);
			this.$emit('update:modelValue', updated);
		},
	},
};
</script>

<style lang="scss">
@import '@/styles/vars.scss';

.topics-input {
	.topics-input-tags {
		flex-wrap: wrap;
	}
	.topics-input-tag {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		padding: 8px 10px;
		border-radius: 10px;
		border: thin solid white;
		background-color: $topic-bg-color;
		color: $topic-fg-color;
		line-height: 1.2;
		white-space: nowrap;

		.remove-btn {
			cursor: pointer;
			font-size: 18px;
		}
	}
}
</style>
