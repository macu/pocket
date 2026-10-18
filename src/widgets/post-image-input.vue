<template>
<div class="post-image-input flex-column-sm">
	<p>JPEG, PNG, or static WebP, up to 10 MiB. Automatically resized and AI-moderated.</p>
	<input ref="fileInput" type="file" accept="image/jpeg,image/png,image/webp" :disabled="disabled" @change="selectImage"/>
	<p v-if="error" class="form-field-error" role="alert">{{error}}</p>
	<img v-if="preview" :src="preview" alt="Post image preview"/>
	<p v-else-if="currentImage && removeCurrent">The current image will be deleted when you save.</p>
	<div class="flex-row-sm">
		<el-button v-if="preview" :disabled="disabled" @click="removeImage">Remove image</el-button>
		<el-button v-if="currentImage && (modelValue || removeCurrent)" :disabled="disabled" @click="restoreImage">
			Restore current image
		</el-button>
	</div>
</div>
</template>

<script>
export default {
	props: {
		modelValue: {type: File, default: null},
		currentImage: {type: String, default: ''},
		removeCurrent: {type: Boolean, default: false},
		disabled: {type: Boolean, default: false},
	},
	emits: ['update:modelValue', 'update:removeCurrent'],
	data() {
		return {objectURL: '', error: ''};
	},
	computed: {
		preview() {
			return this.objectURL || (!this.removeCurrent ? this.currentImage : '');
		},
	},
	watch: {
		modelValue: {
			immediate: true,
			handler(file) {
				if (this.objectURL) URL.revokeObjectURL(this.objectURL);
				this.objectURL = file ? URL.createObjectURL(file) : '';
			},
		},
	},
	beforeUnmount() {
		if (this.objectURL) URL.revokeObjectURL(this.objectURL);
	},
	methods: {
		selectImage(event) {
			const file = event.target.files[0];
			if (!file) return;
			if (!['image/jpeg', 'image/png', 'image/webp'].includes(file.type) || file.size > 10 * 1024 * 1024) {
				this.error = 'Choose a JPEG, PNG, or WebP image no larger than 10 MiB.';
				event.target.value = '';
				return;
			}
			this.error = '';
			this.$emit('update:modelValue', file);
		},
		removeImage() {
			this.$emit('update:modelValue', null);
			this.$emit('update:removeCurrent', !!this.currentImage);
			this.$refs.fileInput.value = '';
			this.error = '';
		},
		restoreImage() {
			this.$emit('update:modelValue', null);
			this.$emit('update:removeCurrent', false);
			this.$refs.fileInput.value = '';
			this.error = '';
		},
	},
};
</script>

<style lang="scss">
.post-image-input {
	img {
		display: block;
		max-width: 100%;
		max-height: 400px;
		object-fit: contain;
		align-self: center;
	}
	input[type="file"] {
		max-width: 100%;
	}
}
</style>
