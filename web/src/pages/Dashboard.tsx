import { useNavigate } from 'react-router-dom'

export function Dashboard() {
  const navigate = useNavigate()

  const handleLogout = async () => {
    await fetch('/api/logout', { method: 'POST', credentials: 'include' })
    navigate('/login', { replace: true })
  }

  return (
    <div className="min-h-screen bg-[#111827] text-white p-8">
      <div className="max-w-4xl mx-auto">
        <div className="flex justify-between items-center mb-8">
          <h1 className="text-2xl font-bold">仪表盘</h1>
          <button
            onClick={handleLogout}
            className="px-4 py-2 rounded-lg bg-gray-700 hover:bg-gray-600"
          >
            退出登录
          </button>
        </div>
        <div className="rounded-lg bg-[#1a1a2e] p-6">
          <p className="text-gray-400">欢迎使用 s-ui 管理面板</p>
        </div>
      </div>
    </div>
  )
}
