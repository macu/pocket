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
import UserPage from '@/pages/user/index.vue';
import AdminPage from '@/pages/admin/index.vue';
import AdminUserPage from '@/pages/admin/user.vue';
import PostPage from '@/pages/posts/post.vue';
import AccountPage from '@/pages/user-account/account.vue';
import ConfirmEmailPage from '@/pages/user-account/confirm-email.vue';
import ResetPasswordPage from '@/pages/user-account/reset-password.vue';
import AboutPage from '@/pages/about/index.vue';

const router = createRouter({
	history: createWebHistory(),
	routes: [
		{
			path: '/',
			name: 'dashboard',
			meta: { title: 'Dashboard' },
			component: DashboardPage,
		},
		{
			path: '/about',
			name: 'about',
			meta: { title: 'About' },
			component: AboutPage,
		},
		{
			path: '/login',
			name: 'login',
			meta: { title: 'Log in' },
			component: LoginPage,
		},
		{
			path: '/signup',
			name: 'signup',
			meta: { title: 'Sign up' },
			component: SignupPage,
		},
		{
			path: '/verify-signup',
			name: 'signup-verify',
			meta: { title: 'Verify signup' },
			component: SignupVerifyPage,
		},
		{
			path: '/account',
			name: 'account',
			meta: { title: 'Account' },
			component: AccountPage,
		},
		{
			path: '/confirm-email',
			name: 'confirm-email',
			meta: { title: 'Confirm email' },
			component: ConfirmEmailPage,
		},
		{
			path: '/reset-password',
			name: 'reset-password',
			meta: { title: 'Reset password' },
			component: ResetPasswordPage,
		},
		{
			path: '/add-post',
			name: 'add-post',
			meta: { title: 'Add post' },
			component: AddPostPage,
		},
		{
			path: '/admin',
			name: 'admin',
			meta: { title: 'Admin dashboard' },
			component: AdminPage,
		},
		{
			path: '/admin/user/:id',
			name: 'admin-user',
			meta: { title: 'Admin manage user' },
			component: AdminUserPage,
		},
		{
			path: '/user/:identifier',
			name: 'user',
			meta: { title: 'User profile' },
			component: UserPage,
		},
		{
			path: '/post/:id',
			name: 'post',
			meta: { title: 'View post' },
			component: PostPage,
		},
		{
			path: '/post/:id/edit',
			name: 'edit-post',
			meta: { title: 'Edit post' },
			component: EditPostPage,
		},
	],
});

router.beforeEach((to, from, next) => {
	store.commit('setLoading', true);
	next();
});

router.afterEach((to, from, failure) => {
	store.commit('setLoading', false);
	if (!failure && typeof window.gtag === 'function' &&
		window.localStorage.getItem('pocketCookieConsent') === 'accepted') {
		window.gtag('event', 'page_view', {
			page_path: to.fullPath,
			page_location: window.location.origin + to.fullPath,
			page_title: to.meta.title || document.title,
		});
	}
});

export default router;
