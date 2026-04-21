export type LoginWindowThemeName = 'light' | 'dark'

type LoginWindowThemeModel = {
  rootClass: 'login-shell--light' | 'login-shell--dark'
  surfaceClass: 'login-window--light' | 'login-window--dark'
}

export const getLoginWindowThemeModel = (
  themeName: LoginWindowThemeName
): LoginWindowThemeModel => {
  if (themeName === 'dark') {
    return {
      rootClass: 'login-shell--dark',
      surfaceClass: 'login-window--dark',
    }
  }

  return {
    rootClass: 'login-shell--light',
    surfaceClass: 'login-window--light',
  }
}
