import { NavLink, Route, Routes } from 'react-router'
import CategoryProvider from './CategoryProvider.tsx'
import { CategoriesIcon, ExplorerIcon, LogoMark, MoonIcon, OverviewIcon, SunIcon } from './components/icons.tsx'
import ScanStatus from './components/ScanStatus.tsx'
import ServerFooter from './components/ServerFooter.tsx'
import Categories from './pages/Categories.tsx'
import Explorer from './pages/Explorer.tsx'
import NotFound from './pages/NotFound.tsx'
import Overview from './pages/Overview.tsx'
import { ThemeContext, useTheme } from './theme.ts'

const nav = [
  { to: '/', label: 'Overview', end: true, icon: OverviewIcon },
  { to: '/categories', label: 'Categories', end: false, icon: CategoriesIcon },
  { to: '/explorer', label: 'Explorer', end: false, icon: ExplorerIcon },
]

export default function App() {
  const { theme, toggle } = useTheme()

  return (
    <ThemeContext value={theme}>
      <CategoryProvider>
        <div className="min-h-dvh">
          <header className="sticky top-0 z-20 border-b border-line bg-page/85 backdrop-blur-md">
            <div className="mx-auto flex max-w-7xl flex-wrap items-center gap-x-3 gap-y-1.5 px-4 py-2.5 sm:gap-x-5">
              <NavLink to="/" className="flex shrink-0 items-center gap-2 text-[17px] font-semibold tracking-tight">
                <LogoMark />
                <span className="max-[380px]:hidden">hoardboard</span>
              </NavLink>
              <nav className="hidden gap-1 sm:flex" aria-label="Pages">
                {nav.map((item) => (
                  <NavLink
                    key={item.to}
                    to={item.to}
                    end={item.end}
                    className={({ isActive }) =>
                      `rounded-lg px-3 py-1.5 text-sm font-medium transition-colors ${
                        isActive ? 'bg-raised text-ink' : 'text-muted hover:text-ink'
                      }`
                    }
                  >
                    {item.label}
                  </NavLink>
                ))}
              </nav>
              <ScanStatus />
              <button
                type="button"
                onClick={toggle}
                className="inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-lg text-muted transition-colors hover:bg-raised hover:text-ink"
                aria-label={`Switch to the ${theme === 'dark' ? 'light' : 'dark'} theme`}
                title={`Switch to the ${theme === 'dark' ? 'light' : 'dark'} theme`}
              >
                {theme === 'dark' ? <SunIcon /> : <MoonIcon />}
              </button>
            </div>
          </header>

          <main className="mx-auto max-w-7xl px-3 pt-4 pb-28 sm:px-4 sm:pt-6 sm:pb-12">
            <Routes>
              <Route index element={<Overview />} />
              <Route path="categories" element={<Categories />} />
              <Route path="categories/:id" element={<Categories />} />
              <Route path="explorer/*" element={<Explorer />} />
              <Route path="*" element={<NotFound />} />
            </Routes>
            <ServerFooter />
          </main>

          {/* On a phone the pages sit in a tab bar at the bottom, in thumb reach. */}
          <nav
            aria-label="Pages"
            className="fixed inset-x-0 bottom-0 z-20 grid grid-cols-3 border-t border-line bg-page/90 pb-[env(safe-area-inset-bottom)] backdrop-blur-md sm:hidden"
          >
            {nav.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                end={item.end}
                className={({ isActive }) =>
                  `flex flex-col items-center gap-0.5 py-2 text-[11px] font-medium ${isActive ? 'text-ink' : 'text-muted'}`
                }
              >
                <item.icon size={22} />
                {item.label}
              </NavLink>
            ))}
          </nav>
        </div>
      </CategoryProvider>
    </ThemeContext>
  )
}
