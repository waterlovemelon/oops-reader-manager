export function getAccessToken() {
  return localStorage.getItem('manager_access_token');
}

export function setAccessToken(token: string) {
  localStorage.setItem('manager_access_token', token);
}

export function clearAccessToken() {
  localStorage.removeItem('manager_access_token');
}
