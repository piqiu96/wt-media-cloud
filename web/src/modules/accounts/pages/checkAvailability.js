export function canCheckAccount({ isDesktop, isOwner, account, windowAvailable }) {
  return Boolean(isDesktop && isOwner && account?.enabled === true && account?.browser_profile_id && windowAvailable)
}
