import { createBrowserRouter, Navigate, redirect } from 'react-router-dom'
import { Setup } from './pages/Setup'
import { Login } from './pages/Login'
import { Dashboard } from './pages/Dashboard'

async function checkAuth() {
  const res = await fetch('/api/me', { credentials: 'include' })
  return res.ok
}

export const router = createBrowserRouter([
  { path: '/setup', element: <Setup /> },
  { path: '/login', element: <Login /> },
  {
    path: '/',
    element: <Dashboard />,
    loader: async () => {
      const ok = await checkAuth()
      if (!ok) return redirect('/login')
      return null
    },
  },
  { path: '*', element: <Navigate to="/" replace /> },
])
