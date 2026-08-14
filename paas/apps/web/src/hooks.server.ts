import { redirect } from '@sveltejs/kit';
import type { Handle } from '@sveltejs/kit';

export const handle: Handle = async ({ event, resolve }) => {
	// Excluded routes from auth redirect
	const publicRoutes = ['/login', '/signup', '/access/pending'];
	
	const isPublicRoute = publicRoutes.some(route => event.url.pathname.startsWith(route));
	const sessionToken = event.cookies.get('session_token') || event.cookies.get('klouds_session');

	if (!sessionToken && !isPublicRoute) {
		throw redirect(302, '/login');
	}

	return await resolve(event);
};
