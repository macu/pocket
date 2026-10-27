const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');
const babel = require('@babel/core');

const { code } = babel.transformSync(
	fs.readFileSync(path.join(__dirname, 'router.js'), 'utf8'),
	{
		babelrc: false,
		configFile: false,
		presets: [['@babel/preset-env', { targets: { node: 'current' } }]],
	},
);

function loadRouter(consent = 'accepted', analyticsAvailable = true) {
	const events = [];
	const commits = [];
	const router = {
		beforeEach(hook) { this.beforeHook = hook; },
		afterEach(hook) { this.afterHook = hook; },
	};
	vm.runInNewContext(code, {
		exports: {},
		require(moduleName) {
			if (moduleName === 'vue-router') {
				return {
					createRouter: (options) => {
						router.routes = options.routes;
						return router;
					},
					createWebHistory: () => ({}),
				};
			}
			if (moduleName === './store.js') {
				return { commit: (...args) => commits.push(args) };
			}
			if (moduleName.endsWith('.vue')) {
				return {};
			}
			throw new Error(`Unexpected import: ${moduleName}`);
		},
		window: {
			location: { origin: 'https://pocket.example' },
			localStorage: {
				getItem(key) {
					assert.equal(key, 'pocketCookieConsent');
					return consent;
				},
			},
			gtag: analyticsAvailable ? (...args) => events.push(args) : undefined,
		},
		document: { title: 'Pocket' },
	});
	return { router, events, commits };
}

for (const fullPath of ['/', '/about', '/post/123', '/user/alice?tab=posts#latest']) {
	test(`reports the GA4 page location for ${fullPath}`, () => {
		const { router, events, commits } = loadRouter();
		router.afterHook({ fullPath, meta: {} }, {});
		assert.equal(events.length, 1);
		const [command, name, parameters] = events[0];
		assert.equal(command, 'event');
		assert.equal(name, 'page_view');
		assert.equal(parameters.page_location, `https://pocket.example${fullPath}`);
		assert.equal(parameters.page_path, fullPath);
		assert.equal(parameters.page_title, 'Pocket');
		assert.deepEqual(commits, [['setLoading', false]]);
	});
}

test('reports the configured title for every route', () => {
	const { router, events } = loadRouter();
	const expectedTitles = {
		dashboard: 'Dashboard',
		about: 'About',
		login: 'Log in',
		signup: 'Sign up',
		'signup-verify': 'Verify signup',
		account: 'Account',
		'confirm-email': 'Confirm email',
		'reset-password': 'Reset password',
		'add-post': 'Add post',
		admin: 'Admin',
		'admin-user': 'Admin user',
		user: 'User profile',
		post: 'Post',
		'edit-post': 'Edit post',
	};
	assert.equal(router.routes.length, Object.keys(expectedTitles).length);
	for (const route of router.routes) {
		assert.equal(route.meta.title, expectedTitles[route.name]);
		router.afterHook({ fullPath: route.path, meta: route.meta }, {});
		assert.equal(events.at(-1)[2].page_title, expectedTitles[route.name]);
	}
	assert.equal(events.length, router.routes.length);
});

for (const consent of [null, 'rejected']) {
	test(`does not track without accepted consent (${consent})`, () => {
		const { router, events, commits } = loadRouter(consent);
		router.afterHook({ fullPath: '/about' }, {});
		assert.equal(events.length, 0);
		assert.deepEqual(commits, [['setLoading', false]]);
	});
}

test('does not track failed navigation', () => {
	const { router, events, commits } = loadRouter();
	router.afterHook({ fullPath: '/about' }, {}, new Error('Navigation failed'));
	assert.equal(events.length, 0);
	assert.deepEqual(commits, [['setLoading', false]]);
});

test('handles analytics being unavailable', () => {
	const { router, events, commits } = loadRouter('accepted', false);
	router.afterHook({ fullPath: '/about' }, {});
	assert.equal(events.length, 0);
	assert.deepEqual(commits, [['setLoading', false]]);
});

test('preserves the navigation loading indicator', () => {
	const { router, commits } = loadRouter();
	let continued = false;
	router.beforeHook({}, {}, () => { continued = true; });
	assert.equal(continued, true);
	assert.deepEqual(commits, [['setLoading', true]]);
});
