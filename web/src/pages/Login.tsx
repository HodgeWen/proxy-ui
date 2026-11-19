import { useState } from "react"
import { ArrowRight } from "lucide-react"
import {
  Alert,
  Button,
  Card,
  Description,
  Form,
  Input,
  Label,
  TextField,
} from "@heroui/react"
import { useNavigate } from "react-router-dom"

export function Login() {
  const navigate = useNavigate()
  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")
  const [error, setError] = useState("")
  const [loading, setLoading] = useState(false)
  const hasCredentialError = error === "用户名或密码错误"

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError("")
    setLoading(true)

    try {
      const res = await fetch("/api/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify({ username: username.trim(), password }),
      })

      if (!res.ok) {
        setError("用户名或密码错误")
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
    <div className="flex min-h-screen items-center justify-center bg-background p-4">
      <Card className="w-full max-w-md ">
        <Card.Header className="flex flex-col items-center gap-4 text-center mt-4">
          <div className="flex size-16 items-center justify-center rounded-2xl bg-primary/10 text-primary">
            <span className="text-2xl font-bold">S</span>
          </div>
          <div className="space-y-1">
            <Card.Title className="text-2xl font-semibold">S-UI</Card.Title>
            <Card.Description className="text-sm text-foreground-500">
              sing-box 管理面板
            </Card.Description>
          </div>
        </Card.Header>

        <Card.Content >
          <Form
            onSubmit={handleSubmit}
            validationBehavior="aria"
            className="space-y-6"
          >
            <TextField
              name="username"
              isRequired
              fullWidth
              isInvalid={hasCredentialError}
              className="space-y-2"
            >
              <Label>用户名</Label>
              <Input
                autoComplete="username"
                autoFocus
                placeholder="输入管理员用户名"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
              />
              <Description>管理员账户名</Description>
            </TextField>

            <TextField
              name="password"
              isRequired
              fullWidth
              isInvalid={hasCredentialError}
              className="space-y-2"
            >
              <Label>密码</Label>
              <Input
                autoComplete="current-password"
                placeholder="输入当前密码"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
              <Description>输入后直接进入控制台</Description>
            </TextField>

            {error && (
              <Alert color="danger">{error}</Alert>
            )}

            <div className="space-y-3">
              <Button
                fullWidth
                isDisabled={loading}
                type="submit"
                variant="primary"
                size="lg"
              >
                {loading ? "登录中..." : "登录"}
              </Button>

              <a
                href="/setup"
                className="flex items-center justify-center gap-2 py-3 text-sm text-foreground-500 transition-colors hover:text-primary"
              >
                <span>首次部署时前往初始化管理员账户</span>
                <ArrowRight className="size-4 shrink-0" />
              </a>
            </div>
          </Form>
        </Card.Content>
      </Card>
    </div>
  )
}
