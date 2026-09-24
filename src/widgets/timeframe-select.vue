<template>
<el-select :model-value="modelValue" @update:model-value="$emit('update:modelValue', $event)">
	<el-option label="Last 24 hours" value="24h"/>
	<el-option label="Last 7 days" value="7d"/>
	<el-option label="Last 30 days" value="30d"/>
	<el-option label="All time" value="all"/>
</el-select>
</template>

<script>
// shared storage key so the selected timeframe carries over between pages
export const TIMEFRAME_STORAGE_KEY = 'timeframe';

export default {
	props: {
		modelValue: {
			type: String,
			default: 'all',
		},
	},
	emits: ['update:modelValue'],
};
</script>
