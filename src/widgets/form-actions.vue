<template>
<div class="form-actions flex-row-md">
	<slot/>
</div>
</template>

<style lang="scss">
@import '@/styles/vars.scss';

.form-actions {
	padding: 12px;
	border: thin solid white;
	border-radius: $border-radius;
}
</style>
