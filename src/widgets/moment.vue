<template>
<el-tooltip v-if="ago" effect="dark"
	:content="tooltipString"
	placement="top-start">
	<span class="moment" v-text="displayString"/>
</el-tooltip>
<span v-else class="moment" v-text="displayString"/>
</template>

<script>
import moment from 'moment';

export default {
	props: {
		time: {
			type: String,
			required: true,
		},
		ago: {
			type: Boolean,
			default: true,
		},
	},
	computed: {
		displayAgo() {
			return moment(this.time).fromNow();
		},
		displayFormatted() {
			return moment(this.time).format('YYYY-MM-DD HH:mm:ss');
		},
		displayString() {
			return this.ago
				? this.displayAgo
				: this.displayFormatted;
		},
		tooltipString() {
			return this.displayFormatted;
		},
	},
};
</script>
