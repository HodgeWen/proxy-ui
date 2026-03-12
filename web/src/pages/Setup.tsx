import { useState } from "react"
import {
  Button,
  Card,
  Description,
  Form,
  Input,
  Label,
  TextField,
} from "@heroui/react"
import { useNavigate } from "react-router-dom"

export function Setup() {
  const navigate = useNavigate()
  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")
  const [confirm, setConfirm] = useState("")
  const [error, setError] = useState("")
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError("")

    if (username.trim().length < 3 || username.trim().length > 50) {
      setError("用户名需 3-50 个字符")
      return
    }

    if (password.length < 8) {
      setError("密码至少 8 个字符")
      return
    }

    if (password !== confirm) {
      setError("两次密码输入不一致")
      return
    }

    setLoading(true)

    try {
      const res = await fetch("/api/setup", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify({ username: username.trim(), password, confirm }),
      })

      if (!res.ok) {
        const text = await res.text()
        setError(text || "设置失败")
        return
      }

      navigate("/", { replace: true })
    } catch {
      setError("网络错误")
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-[color:var(--background)] px-4">
      <Card className="w-full max-w-md border border-[color:var(--border)] bg-[color:var(--surface)] shadow-[var(--overlay-shadow)]">
        <Card.Header className="px-6 pt-8 sm:px-8">
          <div className="space-y-1">
            <Card.Title className="text-2xl font-semibold">
              创建管理员账户
            </Card.Title>
            <Card.Description className="text-sm text-[color:var(--muted)]">
              首次启动需创建管理员账户，完成后自动进入面板。
            </Card.Description>
          </div>
        </Card.Header>

        <Card.Content className="px-6 pb-8 sm:px-8">
          <Form
            onSubmit={handleSubmit}
            validationBehavior="aria"
            className="space-y-5"
          >
            <TextField name="username" isRequired fullWidth className="space-y-2">
              <Label>用户名</Label>
              <Input
                autoComplete="username"
                autoFocus
                placeholder="输入管理员用户名"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                variant="secondary"
                className="h-12"
              />
              <Description>长度 3-50 个字符</Description>
            </TextField>

            <div className="grid gap-5 sm:grid-cols-2">
              <TextField name="password" isRequired fullWidth className="space-y-2">
                <Label>密码</Label>
                <Input
                  autoComplete="new-password"
                  placeholder="至少 8 个字符"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  variant="secondary"
                  className="h-12"
                />
                <Description>建议使用高强度密码</Description>
              </TextField>

              <TextField name="confirm" isRequired fullWidth className="space-y-2">
                <Label>确认密码</Label>
                <Input
                  autoComplete="new-password"
                  placeholder="再次输入密码"
                  type="password"
                  value={confirm}
                  onChange={(e) => setConfirm(e.target.value)}
                  variant="secondary"
                  className="h-12"
                />
                <Description>需与上方密码一致</Description>
              </TextField>
            </div>

            {error && (
              <div className="rounded-xl border border-[color:var(--danger)]/30 bg-[color:var(--danger)]/10 px-4 py-3 text-sm text-[color:var(--danger)]">
                {error}
              </div>
            )}

            <div className="space-y-3">
              <Button
                fullWidth
                isDisabled={loading}
                type="submit"
                variant="primary"
                className="h-12 text-base font-semibold shadow-[var(--surface-shadow)]"
              >
                {loading ? "设置中..." : "完成设置"}
              </Button>

              <p className="text-center text-sm text-[color:var(--muted)]">
                提交成功后自动登录当前浏览器
              </p>
            </div>
          </Form>
        </Card.Content>
      </Card>
    </div>
  )
}
