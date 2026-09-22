import {
	createRouter,
	createWebHistory,
} from 'vue-router';

import store from './store.js';

import DashboardPage from '@/pages/dashboard/index.vue';
import LoginPage from '@/pages/user-account/login.vue';
import SignupPage from '@/pages/user-account/signup.vue';
import SignupVerifyPage from '@/pages/user-account/signup-verify.vue';
import AddPostPage from '@/pages/posts/add-post.vue';
import EditPostPage from '@/pages/posts/edit-post.vue';
import PostPage from '@/pages/posts/post.vue';

const router = createRouter({
	history: createWebHistory(),
	routes: [
		{
			path: '/',
			name: 'dashboard',
			component: DashboardPage,
		},
		{
			path: '/login',
			name: 'login',
			component: LoginPage,
		},
		{
			path: '/signup',
			name: 'signup',
			component: SignupPage,
		},
		{
			path: '/verify-signup',
			name: 'signup-verify',
			component: SignupVerifyPage,
		},
		{
			path: '/add-post',
			name: 'add-post',
			component: AddPostPage,
		},
		{
			path: '/post/:id',
			name: 'post',
			component: PostPage,
		},
		{
			path: '/post/:id/edit',
			name: 'edit-post',
			component: EditPostPage,
		},
	],
});

router.beforeEach((to, from, next) => {
	store.commit('setLoading', true);
	next();
});

router.afterEach(() => {
	store.commit('setLoading', false);
});

export default router;
