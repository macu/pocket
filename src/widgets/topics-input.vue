<template>
<div class="topics-input flex-column-sm">
	<div class="topics-input-row flex-row-sm">
		<el-input ref="input"
			v-model="pendingValue"
			type="text" size="large"
			:maxlength="$const.maxTopicLength"
			:disabled="atMax"
			:placeholder="atMax ? 'Max topics reached' : 'Add a topic'"
			@input="scheduleSearch()"
			@focus="showSuggestions = true"
			@blur="hideSuggestionsDelayed()"
			@keydown.down.prevent.native="moveActiveSuggestion(1)"
			@keydown.up.prevent.native="moveActiveSuggestion(-1)"
			@keyup.enter.native="addPending()">
			<template #append>
				<el-button @click="addPending()" type="primary" :disabled="addDisabled">Add</el-button>
			</template>
		</el-input>
		<ul v-if="showSuggestions && suggestions.length" class="topics-input-suggestions">
			<li v-for="(suggestion, index) in suggestions" :key="suggestion.id"
				:class="{active: index === activeSuggestionIndex}"
				@mousedown.prevent="selectSuggestion(suggestion)">
				{{suggestion.name}}
			</li>
		</ul>
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

import {
	ajaxGet,
} from '@/utils/ajax.js';

const SEARCH_DEBOUNCE_MS = 250;

export default {
	components: {
		MaterialIcon,
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
			suggestions: [],
			showSuggestions: false,
			searchTimeout: null,
			activeSuggestionIndex: -1,
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
			if (this.$refs.input && this.$refs.input.focus) {
				this.$refs.input.focus();
			}
		},
		scheduleSearch() {
			clearTimeout(this.searchTimeout);
			const query = this.pendingValue.trim();
			if (!query) {
				this.suggestions = [];
				this.activeSuggestionIndex = -1;
				return;
			}
			this.searchTimeout = setTimeout(() => {
				ajaxGet('/ajax/topics/search', {query}).then(response => {
					this.suggestions = response.topics || [];
					this.activeSuggestionIndex = -1;
					this.showSuggestions = true;
				});
			}, SEARCH_DEBOUNCE_MS);
		},
		moveActiveSuggestion(delta) {
			if (!this.showSuggestions || !this.suggestions.length) {
				return;
			}
			const maxIndex = this.suggestions.length - 1;
			let next = this.activeSuggestionIndex + delta;
			if (next < 0) {
				next = maxIndex;
			} else if (next > maxIndex) {
				next = 0;
			}
			this.activeSuggestionIndex = next;
		},
		hideSuggestionsDelayed() {
			// delay so a suggestion's mousedown/click can register before the dropdown is hidden
			setTimeout(() => {
				this.showSuggestions = false;
			}, 150);
		},
		selectSuggestion(topic) {
			this.addName(topic.name);
			this.suggestions = [];
			this.showSuggestions = false;
			this.activeSuggestionIndex = -1;
		},
		addPending() {
			if (this.activeSuggestionIndex >= 0 && this.suggestions[this.activeSuggestionIndex]) {
				this.selectSuggestion(this.suggestions[this.activeSuggestionIndex]);
				return;
			}
			this.addName(this.pendingValue);
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
			this.suggestions = [];
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
		position: relative;
	}
	.topics-input-suggestions {
		position: absolute;
		top: 100%;
		left: 0;
		z-index: 10;
		margin: 4px 0 0;
		padding: 4px 0;
		list-style: none;
		min-width: 200px;
		max-height: 240px;
		overflow-y: auto;
		border-radius: $border-radius;
		background-color: $topic-bg-color;
		color: $topic-fg-color;
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);

		li {
			padding: 8px 12px;
			cursor: pointer;

			&:hover, &.active {
				background-color: rgba(255, 255, 255, 0.1);
			}
		}
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
