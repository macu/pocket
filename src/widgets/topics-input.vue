<template>
<div class="topics-input flex-column-sm">
	<div class="topics-input-row flex-row-sm">
		<el-input ref="input"
			v-model="pendingValue"
			type="text" size="large"
			:maxlength="$const.maxTopicLength"
			placeholder="Add a topic"
			@keyup.enter.native="addPending()">
			<template #append>
				<el-button @click="addPending()" type="primary" :disabled="addDisabled">Add</el-button>
			</template>
		</el-input>
	</div>
	<div v-if="modelValue.length" class="topics-input-tags flex-row-sm">
		<span v-for="(name, index) in modelValue" :key="name" class="topics-input-tag">
			{{name}}
			<material-icon icon="close" class="remove-btn" @click.stop="removeAt(index)"/>
		</span>
	</div>
</div>
</template>

<script>
import MaterialIcon from '@/widgets/material-icon.vue';

export default {
	components: {
		MaterialIcon,
	},
	props: {
		modelValue: {
			type: Array,
			default: () => [],
		},
	},
	emits: ['update:modelValue'],
	data() {
		return {
			pendingValue: '',
		};
	},
	computed: {
		addDisabled() {
			return !this.pendingValue.trim();
		},
	},
	methods: {
		focus() {
			if (this.$refs.input && this.$refs.input.focus) {
				this.$refs.input.focus();
			}
		},
		addPending() {
			const name = this.pendingValue.trim();
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
	.topics-input-row {
		align-items: center;
	}
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
