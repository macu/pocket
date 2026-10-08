<template>
<div class="topic-search-row flex-row-sm">
	<el-input ref="input"
		:model-value="modelValue"
		type="text" size="large"
		:maxlength="$const.maxTopicLength"
		:disabled="disabled"
		:placeholder="placeholder"
		@input="scheduleSearch"
		@focus="showSuggestions = true"
		@blur="hideSuggestionsDelayed()"
		@keydown.down.prevent.native="moveActiveSuggestion(1)"
		@keydown.up.prevent.native="moveActiveSuggestion(-1)"
		@keyup.enter.native="submit()">
		<template v-if="$slots.append" #append>
			<slot name="append"/>
		</template>
	</el-input>
	<ul v-if="showSuggestions && suggestions.length" class="topic-search-suggestions">
		<li v-for="(suggestion, index) in suggestions" :key="suggestion.id"
			:class="{active: index === activeSuggestionIndex}"
			@mousedown.prevent="selectSuggestion(suggestion)">
			{{suggestion.name}}
		</li>
	</ul>
</div>
</template>

<script>
import {
	ajaxGet,
} from '@/utils/ajax.js';

const SEARCH_DEBOUNCE_MS = 250;

export default {
	props: {
		modelValue: {
			type: String,
			default: '',
		},
		placeholder: {
			type: String,
			default: 'Search topics',
		},
		disabled: {
			type: Boolean,
			default: false,
		},
		excludeIds: {
			type: Array,
			default: () => [],
		},
	},
	emits: ['update:modelValue', 'select', 'submit'],
	data() {
		return {
			searchResults: [],
			showSuggestions: false,
			searchTimeout: null,
			searchRequestId: 0,
			activeSuggestionIndex: -1,
		};
	},
	computed: {
		suggestions() {
			const excludedIds = new Set(this.excludeIds);
			return this.searchResults.filter(topic => !excludedIds.has(topic.id));
		},
	},
	beforeUnmount() {
		clearTimeout(this.searchTimeout);
	},
	methods: {
		focus() {
			if (this.$refs.input && this.$refs.input.focus) {
				this.$refs.input.focus();
			}
		},
		scheduleSearch(value) {
			this.$emit('update:modelValue', value);
			clearTimeout(this.searchTimeout);
			const requestId = ++this.searchRequestId;
			const query = value.trim();
			this.searchResults = [];
			this.activeSuggestionIndex = -1;
			if (!query) {
				return;
			}
			this.searchTimeout = setTimeout(() => {
				ajaxGet('/ajax/topics/search', {query}).then(response => {
					if (requestId !== this.searchRequestId) {
						return;
					}
					this.searchResults = response.topics || [];
					this.showSuggestions = true;
				}).catch(() => {
					// ajaxGet already displays a request error.
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
			setTimeout(() => {
				this.showSuggestions = false;
			}, 150);
		},
		selectSuggestion(topic) {
			clearTimeout(this.searchTimeout);
			this.searchRequestId++;
			this.searchResults = [];
			this.showSuggestions = false;
			this.activeSuggestionIndex = -1;
			this.$emit('select', topic);
		},
		submit() {
			if (this.activeSuggestionIndex >= 0 && this.suggestions[this.activeSuggestionIndex]) {
				this.selectSuggestion(this.suggestions[this.activeSuggestionIndex]);
				return;
			}
			this.$emit('submit');
		},
	},
};
</script>

<style lang="scss">
@import '@/styles/vars.scss';

.topic-search-row {
	align-items: center;
	position: relative;
	width: 100%;

	.topic-search-suggestions {
		display: flex;
		flex-direction: column;
		position: absolute;
		top: 100%;
		left: 0;
		z-index: 1000;
		margin: 4px 0 0;
		padding: 6px;
		gap: 4px;
		list-style: none;
		width: 100%;
		min-width: 200px;
		max-height: 240px;
		overflow-y: auto;
		box-sizing: border-box;
		border: 1px solid rgba(255, 255, 255, 0.25);
		border-radius: $border-radius;
		background-color: $suggestions-bg-color;
		color: $app-fg-color;
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.6);

		li {
			padding: 8px 12px;
			border-radius: $border-radius;
			cursor: pointer;

			&:hover, &.active {
				background-color: rgba(255, 255, 255, 0.12);
			}
		}
	}
}
</style>
