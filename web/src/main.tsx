import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Toast } from '@heroui/react'
import { ThemeProvider } from '@/components/theme-provider'
import './index.css'
import App from './App.tsx'

const queryClient = new QueryClient()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ThemeProvider defaultTheme="dark" storageKey="s-ui-theme">
      <QueryClientProvider client={queryClient}>
        <App />
        <Toast.Provider placement="top" maxVisibleToasts={4} />
      </QueryClientProvider>
    </ThemeProvider>
  </StrictMode>,
)
