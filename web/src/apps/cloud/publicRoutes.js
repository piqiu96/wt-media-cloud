const PUBLIC_PATHS = new Set(['/home', '/login'])

export function isPublicCloudRoute(path) {
  return PUBLIC_PATHS.has(path)
}
