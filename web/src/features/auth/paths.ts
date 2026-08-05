/**
 * The routes the identity screens live at, as constants.
 *
 * They are here rather than inline because three different things have to agree
 * on them — the route table, the guards that redirect to them, and the nav that
 * links to them — and a fourth spelling of `/settings/account` is a link that
 * silently lands on the 404 page.
 */
export const LOGIN_PATH = '/login'
export const MFA_CHALLENGE_PATH = '/login/mfa'
export const ENROLMENT_PATH = '/login/enrol'

export const ACCOUNT_PATH = '/settings/account'
export const TOKENS_PATH = '/settings/tokens'

export const ADMIN_USERS_PATH = '/admin/users'
export const ADMIN_ACTIVITY_PATH = '/admin/activity'
