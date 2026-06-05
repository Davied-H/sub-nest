import { Fragment, useCallback, useEffect, useMemo, useState } from "react"
import {
  ActivityIcon,
  ArchiveRestoreIcon,
  CheckCircle2Icon,
  ClipboardIcon,
  DatabaseBackupIcon,
  DownloadIcon,
  EyeIcon,
  FileJsonIcon,
  KeyRoundIcon,
  Loader2Icon,
  LockIcon,
  PauseCircleIcon,
  PlusIcon,
  RefreshCcwIcon,
  SaveIcon,
  ServerIcon,
  UserIcon,
  ShieldIcon,
  Trash2Icon,
  WifiOffIcon,
} from "lucide-react"
import { FiChevronDown, FiChevronRight } from "react-icons/fi"
import { toast } from "sonner"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Checkbox } from "@/components/ui/checkbox"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@/components/ui/empty"
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Separator } from "@/components/ui/separator"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { Skeleton } from "@/components/ui/skeleton"
import { Switch } from "@/components/ui/switch"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Textarea } from "@/components/ui/textarea"
import { Toaster } from "@/components/ui/sonner"
import { TooltipProvider } from "@/components/ui/tooltip"

type Dashboard = {
  sourceCount: number
  enabledSources: number
  healthySources: number
  unhealthySources: number
  outputCount: number
  enabledOutputs: number
  totalCachedNodes: number
  lastRefreshAt?: string
  publicExampleUrl: string
  needsAdminSetup: boolean
}

type Source = {
  id: string
  name: string
  url?: string
  sourceType?: "url" | "file"
  fileName?: string
  fileContent?: string
  urlMasked: string
  enabled: boolean
  remark: string
  tags: string[]
  lastStatus: string
  lastFormat: string
  lastNodeCount: number
  lastError: string
  refreshProgress: string
  refreshPercent: number
  lastRefreshedAt?: string
  lastSuccessAt?: string
  nodeStats: {
    total: number
    alive: number
    dead: number
    unchecked: number
  }
  nodes?: Array<{
    key: string
    name: string
    originalName: string
    server: string
    port: number
    region: string
    regionCode: string
    resolvedIp: string
    exitIp: string
    alive?: boolean
    excludedReason: string
    regionSource: string
    probeStatus: string
    probeError: string
  }>
}

type Output = {
  id: string
  slug: string
  name: string
  enabled: boolean
  format: string
  sourceIds: string[]
  filter: {
    includeKeywords: string[]
    excludeKeywords: string[]
    regex: string
  }
  renameRules: Array<{ pattern: string; replacement: string }>
  nodeNameOverrides?: Record<string, string>
  groupMode: string
  lastGeneratedAt?: string
  lastNodeCount: number
  lastDroppedCount: number
}

type Preview = {
  outputId: string
  slug: string
  nodeCount: number
  duplicateCount: number
  filteredCount: number
  unavailableCount: number
  regionCounts: Record<string, number>
  groups: Array<{ name: string; nodes: string[] }>
  nodes: Array<{
    key: string
    name: string
    originalName: string
    server: string
    port: number
    region: string
    regionCode: string
    resolvedIp: string
    exitIp: string
    alive?: boolean
    excludedReason: string
    regionSource: string
    probeStatus: string
    probeError: string
  }>
  excludedNodes: Preview["nodes"]
  generatedAt: string
  usedCachedSources: number
}

type Health = {
  ok: boolean
  needsAdminSetup: boolean
}

type Settings = {
  publicBaseUrl: string
  hasUserToken: boolean
}

type User = {
  id: string
  slug: string
  name: string
  role: "admin" | "user"
  enabled: boolean
  createdAt?: string
  lastLoginAt?: string
}

type InviteCode = {
  id: string
  label: string
  createdAt: string
  usedAt?: string
  usedByUserId?: string
}

type CreatedInviteCode = InviteCode & {
  code: string
}

const defaultSource: Source = {
  id: "",
  name: "",
  url: "",
  sourceType: "url",
  fileName: "",
  fileContent: "",
  urlMasked: "",
  enabled: true,
  remark: "",
  tags: [],
  lastStatus: "pending",
  lastFormat: "",
  lastNodeCount: 0,
  lastError: "",
  refreshProgress: "",
  refreshPercent: 0,
  nodeStats: { total: 0, alive: 0, dead: 0, unchecked: 0 },
  nodes: [],
}

const defaultOutput: Output = {
  id: "",
  slug: "main",
  name: "主订阅",
  enabled: true,
  format: "clash",
  sourceIds: [],
  filter: {
    includeKeywords: [],
    excludeKeywords: ["官网", "剩余", "过期", "流量"],
    regex: "",
  },
  renameRules: [],
  nodeNameOverrides: {},
  groupMode: "region",
  lastNodeCount: 0,
  lastDroppedCount: 0,
}

const downloadFormatItems = [
  { label: "Mihomo YAML", value: "mihomo", filename: "mihomo.yaml" },
  { label: "Clash YAML", value: "clash", filename: "clash.yaml" },
  { label: "Base64 分享链接", value: "base64", filename: "subscription.txt" },
]

const outputFormatItems = [
  { label: "Clash / Mihomo YAML", value: "clash" },
  { label: "Base64 分享链接", value: "base64" },
]

function App() {
  const [health, setHealth] = useState<Health | null>(null)
  const [token, setToken] = useState(() => localStorage.getItem("sub-nest-token") ?? localStorage.getItem("subagg-token") ?? "")
  const [user, setUser] = useState<User | null>(() => readStoredUser())
  const [users, setUsers] = useState<User[]>([])
  const [targetUserId, setTargetUserId] = useState(() => localStorage.getItem("sub-nest-target-user") ?? "")
  const [dashboard, setDashboard] = useState<Dashboard | null>(null)
  const [sources, setSources] = useState<Source[]>([])
  const [outputs, setOutputs] = useState<Output[]>([])
  const [preview, setPreview] = useState<Preview | null>(null)
  const [settings, setSettings] = useState<Settings | null>(null)
  const [userToken, setUserToken] = useState(() => localStorage.getItem("sub-nest-user-token") ?? "")
  const [activeTab, setActiveTab] = useState("overview")
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState("")

  const api = useMemo(() => createAPI(token), [token])
  const authenticated = Boolean(token)
  const targetUserKnown = users.some((item) => item.id === targetUserId)
  const activeUserId = user?.role === "admin" ? (targetUserKnown ? targetUserId : user.id) : user?.id
  const activeUser = users.find((item) => item.id === activeUserId) ?? user
  const scopeQuery = user?.role === "admin" && activeUserId && activeUserId !== user.id ? `?userId=${encodeURIComponent(activeUserId)}` : ""
  const anyRefreshing = sources.some((source) => source.lastStatus === "refreshing")

  const loadProtected = useCallback(async (options?: { silent?: boolean }) => {
    if (!token) {
      return
    }
    if (!options?.silent) {
      setLoading(true)
    }
    try {
      const [nextUserResponse, nextUsers, nextDashboard, nextSources, nextOutputs, nextSettings] = await Promise.all([
        api.get<{ user: User }>("/api/me"),
        user?.role === "admin" ? api.get<User[]>("/api/admin/users") : Promise.resolve([]),
        api.get<Dashboard>(withScope("/api/dashboard", scopeQuery)),
        api.get<Source[]>(withScope("/api/sources?includeUrl=1", scopeQuery)),
        api.get<Output[]>(withScope("/api/outputs", scopeQuery)),
        api.get<Settings>("/api/settings"),
      ])
      setUser(nextUserResponse.user)
      localStorage.setItem("sub-nest-user", JSON.stringify(nextUserResponse.user))
      if (nextUserResponse.user.role === "admin") {
        const adminUsers = nextUsers.length > 0 ? nextUsers : [nextUserResponse.user]
        setUsers(adminUsers)
        if (!targetUserId || !adminUsers.some((item) => item.id === targetUserId)) {
          setTargetUserId(nextUserResponse.user.id)
          localStorage.setItem("sub-nest-target-user", nextUserResponse.user.id)
        }
      } else {
        setUsers([])
        setTargetUserId("")
        localStorage.removeItem("sub-nest-target-user")
      }
      setDashboard(nextDashboard)
      setSources(nextSources)
      setOutputs(nextOutputs)
      setSettings(nextSettings)
      if (!nextSettings.hasUserToken && userToken) {
        localStorage.removeItem("sub-nest-user-token")
        setUserToken("")
      }
      if (nextOutputs[0]) {
        try {
          setPreview(await api.get<Preview>(withScope(`/api/outputs/${nextOutputs[0].id}/preview`, scopeQuery)))
        } catch {
          setPreview(null)
        }
      }
    } catch (error) {
      localStorage.removeItem("subagg-token")
      localStorage.removeItem("sub-nest-token")
      localStorage.removeItem("sub-nest-user")
      localStorage.removeItem("sub-nest-target-user")
      setToken("")
      setUser(null)
      toast.error(messageOf(error))
    } finally {
      if (!options?.silent) {
        setLoading(false)
      }
    }
  }, [api, scopeQuery, targetUserId, token, user?.id, user?.role, userToken])

  useEffect(() => {
    createAPI("").get<Health>("/api/health").then(setHealth).catch(() => {
      setHealth({ ok: false, needsAdminSetup: false })
    })
  }, [])

  useEffect(() => {
    void loadProtected()
  }, [loadProtected])

  async function handleAuth(rawToken: string, setup: boolean, publicBaseURL: string) {
    setBusy("auth")
    try {
      const response = await createAPI("").post<{ token: string; user: User }>(
        setup ? "/api/setup" : "/api/login",
        setup ? { token: rawToken, publicBaseUrl: publicBaseURL } : { token: rawToken },
      )
      localStorage.setItem("sub-nest-token", response.token)
      localStorage.setItem("sub-nest-user", JSON.stringify(response.user))
      localStorage.setItem("sub-nest-target-user", response.user.id)
      localStorage.removeItem("subagg-token")
      setToken(response.token)
      setUser(response.user)
      setTargetUserId(response.user.id)
      setHealth({ ok: true, needsAdminSetup: false })
      toast.success(setup ? "管理员 token 已设置" : "登录成功")
    } catch (error) {
      toast.error(messageOf(error))
    } finally {
      setBusy("")
    }
  }

  async function handleRegister(payload: { inviteCode: string; userSlug: string; name: string; token: string }) {
    setBusy("auth")
    try {
      const response = await createAPI("").post<{ token: string; user: User }>("/api/register", payload)
      localStorage.setItem("sub-nest-token", response.token)
      localStorage.setItem("sub-nest-user", JSON.stringify(response.user))
      localStorage.removeItem("subagg-token")
      localStorage.removeItem("sub-nest-target-user")
      setToken(response.token)
      setUser(response.user)
      setTargetUserId("")
      setHealth({ ok: true, needsAdminSetup: false })
      toast.success("注册成功")
    } catch (error) {
      toast.error(messageOf(error))
    } finally {
      setBusy("")
    }
  }

  async function refreshAll() {
    setBusy("refresh-all")
    try {
      await api.post(withScope("/api/refresh", scopeQuery), {})
      toast.success("刷新任务已开始")
      await loadProtected()
    } catch (error) {
      toast.error(messageOf(error))
    } finally {
      setBusy("")
    }
  }

  if (!health) {
    return <LoadingShell />
  }

  if (!authenticated) {
    return (
      <TooltipProvider>
        <AuthScreen
          setup={health.needsAdminSetup}
          busy={busy === "auth"}
          onSubmit={handleAuth}
          onRegister={handleRegister}
        />
        <Toaster />
      </TooltipProvider>
    )
  }

  return (
    <TooltipProvider>
      <main className="min-h-screen bg-background text-foreground">
        <div className="mx-auto flex min-h-screen w-full max-w-[1440px] flex-col gap-4 px-4 py-4 sm:px-6 lg:px-8">
          <header className="flex flex-col gap-3 border-b pb-4 lg:flex-row lg:items-center lg:justify-between">
            <div className="flex min-w-0 items-center gap-3">
              <div className="flex size-9 items-center justify-center rounded-lg border bg-muted">
                <ShieldIcon />
              </div>
              <div className="min-w-0">
                <h1 className="truncate text-xl font-semibold">Sub Nest</h1>
                <p className="truncate text-sm text-muted-foreground">
                  固定公开地址，后台维护多个上游订阅源
                </p>
              </div>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <Badge variant="secondary">
                {dashboard?.enabledSources ?? 0} 个启用源
              </Badge>
              <Badge variant="outline">
                {dashboard?.totalCachedNodes ?? 0} 个缓存节点
              </Badge>
              {user ? (
                <Badge variant={user.role === "admin" ? "default" : "secondary"}>
                  <UserIcon />
                  {activeUser?.name || user.name}
                  {user.role === "admin" && activeUser?.id !== user.id ? " 的空间" : ""}
                </Badge>
              ) : null}
              <Button variant="outline" onClick={refreshAll} disabled={busy === "refresh-all" || anyRefreshing}>
                {busy === "refresh-all" || anyRefreshing ? <Loader2Icon data-icon="inline-start" /> : <RefreshCcwIcon data-icon="inline-start" />}
                {anyRefreshing ? "刷新中" : "刷新全部"}
              </Button>
              <Button
                variant="ghost"
                onClick={() => {
                  localStorage.removeItem("subagg-token")
                  localStorage.removeItem("sub-nest-token")
                  localStorage.removeItem("sub-nest-user")
                  localStorage.removeItem("sub-nest-target-user")
                  setToken("")
                  setUser(null)
                  setUsers([])
                  setTargetUserId("")
                  toast.success("已退出")
                }}
              >
                退出
              </Button>
            </div>
          </header>

          <Tabs value={activeTab} onValueChange={setActiveTab} className="min-h-0">
            <div className="flex flex-col gap-3 lg:flex-row">
              <TabsList className="w-full justify-start overflow-x-auto lg:w-fit">
                <TabsTrigger value="overview">首页</TabsTrigger>
                <TabsTrigger value="sources">订阅源</TabsTrigger>
                <TabsTrigger value="outputs">公开订阅</TabsTrigger>
                <TabsTrigger value="preview">预览</TabsTrigger>
                {user?.role === "admin" ? <TabsTrigger value="users">用户</TabsTrigger> : null}
                <TabsTrigger value="backup">备份</TabsTrigger>
                {user?.role === "admin" ? <TabsTrigger value="settings">设置</TabsTrigger> : null}
              </TabsList>
              {user?.role === "admin" ? (
                <Select
                  items={users.map((item) => ({ label: `${item.name || item.slug}${item.enabled ? "" : "（已禁用）"}`, value: item.id }))}
                  value={activeUserId}
                  onValueChange={(value) => {
                    const next = String(value)
                    setTargetUserId(next)
                    localStorage.setItem("sub-nest-target-user", next)
                    setPreview(null)
                  }}
                >
                  <SelectTrigger className="w-full lg:w-56">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      {users.map((item) => (
                        <SelectItem key={item.id} value={item.id}>
                          {item.name || item.slug}{item.enabled ? "" : "（已禁用）"}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              ) : null}
            </div>

            <TabsContent value="overview">
              <Overview
                dashboard={dashboard}
                loading={loading}
                outputs={outputs}
                publicBase={publicBaseFromExample(dashboard?.publicExampleUrl)}
                userToken={userToken}
              />
            </TabsContent>
            <TabsContent value="sources">
              <SourcesView
                api={api}
                sources={sources}
                loading={loading}
                busy={busy}
                setBusy={setBusy}
                scopeQuery={scopeQuery}
                reload={loadProtected}
              />
            </TabsContent>
            <TabsContent value="outputs">
              <OutputsView
                api={api}
                outputs={outputs}
                sources={sources}
                loading={loading}
                scopeQuery={scopeQuery}
                reload={loadProtected}
                publicBase={publicBaseFromExample(dashboard?.publicExampleUrl)}
                userToken={userToken}
              />
            </TabsContent>
            <TabsContent value="preview">
              <PreviewView
                api={api}
                outputs={outputs}
                preview={preview}
                setPreview={setPreview}
                scopeQuery={scopeQuery}
              />
            </TabsContent>
            {user?.role === "admin" ? (
              <TabsContent value="users">
                <UsersView api={api} users={users} reload={loadProtected} />
              </TabsContent>
            ) : null}
            <TabsContent value="backup">
              <BackupView api={api} reload={loadProtected} />
            </TabsContent>
            {user?.role === "admin" ? (
              <TabsContent value="settings">
                <SettingsView
                  api={api}
                  settings={settings}
                  userToken={userToken}
                  onAdminTokenUpdated={(nextToken) => {
                    localStorage.setItem("sub-nest-token", nextToken)
                    localStorage.removeItem("subagg-token")
                    setToken(nextToken)
                  }}
                  onUserTokenUpdated={(nextSettings, nextUserToken) => {
                    setSettings(nextSettings)
                    if (nextUserToken) {
                      localStorage.setItem("sub-nest-user-token", nextUserToken)
                      setUserToken(nextUserToken)
                    } else {
                      localStorage.removeItem("sub-nest-user-token")
                      setUserToken("")
                    }
                  }}
                />
              </TabsContent>
            ) : null}
          </Tabs>
        </div>
      </main>
      <Toaster />
    </TooltipProvider>
  )
}

function AuthScreen({
  setup,
  busy,
  onSubmit,
  onRegister,
}: {
  setup: boolean
  busy: boolean
  onSubmit: (token: string, setup: boolean, publicBaseURL: string) => void
  onRegister: (payload: { inviteCode: string; userSlug: string; name: string; token: string }) => void
}) {
  const [rawToken, setRawToken] = useState("")
  const [mode, setMode] = useState<"login" | "register">("login")
  const [inviteCode, setInviteCode] = useState("")
  const [userSlug, setUserSlug] = useState("")
  const [name, setName] = useState("")
  const [publicBaseURL, setPublicBaseURL] = useState(window.location.origin)
  const registering = !setup && mode === "register"

  return (
    <main className="flex min-h-screen items-center justify-center bg-background p-4">
      <Card className="w-full max-w-md">
        <CardHeader>
          <div className="flex size-10 items-center justify-center rounded-lg border bg-muted">
            {registering ? <KeyRoundIcon /> : <LockIcon />}
          </div>
          <CardTitle>{setup ? "初始化后台访问" : registering ? "使用授权码注册" : "登录后台"}</CardTitle>
          <CardDescription>
            {registering ? "注册后可使用自己的 token 独立维护订阅源和公开订阅。" : "后台使用本地 token 保护；订阅链接会在列表和日志中隐藏敏感部分。"}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <FieldGroup>
            {!setup ? (
              <div className="grid grid-cols-2 gap-2">
                <Button variant={!registering ? "default" : "outline"} onClick={() => setMode("login")}>登录</Button>
                <Button variant={registering ? "default" : "outline"} onClick={() => setMode("register")}>注册</Button>
              </div>
            ) : null}
            {registering ? (
              <>
                <Field>
                  <FieldLabel htmlFor="invite-code">授权码</FieldLabel>
                  <Input id="invite-code" value={inviteCode} onChange={(event) => setInviteCode(event.target.value)} />
                </Field>
                <Field>
                  <FieldLabel htmlFor="user-slug">用户标识</FieldLabel>
                  <Input id="user-slug" value={userSlug} onChange={(event) => setUserSlug(event.target.value)} placeholder="例如 alice" />
                  <FieldDescription>仅使用小写字母、数字和短横线，用于生成 `/u/{userSlug || "alice"}/s/main`。</FieldDescription>
                </Field>
                <Field>
                  <FieldLabel htmlFor="display-name">显示名称</FieldLabel>
                  <Input id="display-name" value={name} onChange={(event) => setName(event.target.value)} placeholder="可选" />
                </Field>
              </>
            ) : null}
            <Field>
              <FieldLabel htmlFor="admin-token">{registering ? "个人 token" : "管理 token"}</FieldLabel>
              <Input
                id="admin-token"
                type="password"
                value={rawToken}
                onChange={(event) => setRawToken(event.target.value)}
                placeholder="至少 8 位"
              />
            </Field>
            {setup ? (
              <Field>
                <FieldLabel htmlFor="public-base">公开访问域名</FieldLabel>
                <Input
                  id="public-base"
                  value={publicBaseURL}
                  onChange={(event) => setPublicBaseURL(event.target.value)}
                />
                <FieldDescription>用于生成复制链接，可稍后在配置文件中调整。</FieldDescription>
              </Field>
            ) : null}
          </FieldGroup>
        </CardContent>
        <CardFooter>
          <Button
            className="w-full"
            disabled={busy || rawToken.length < 8 || (registering && (!inviteCode.trim() || !userSlug.trim()))}
            onClick={() => {
              if (registering) {
                onRegister({ inviteCode, userSlug, name, token: rawToken })
                return
              }
              onSubmit(rawToken, setup, publicBaseURL)
            }}
          >
            {busy ? <Loader2Icon data-icon="inline-start" /> : <ShieldIcon data-icon="inline-start" />}
            {setup ? "完成初始化" : registering ? "创建账号" : "进入后台"}
          </Button>
        </CardFooter>
      </Card>
    </main>
  )
}

function Overview({
  dashboard,
  loading,
  outputs,
  publicBase,
  userToken,
}: {
  dashboard: Dashboard | null
  loading: boolean
  outputs: Output[]
  publicBase: string
  userToken: string
}) {
  const stats = [
    { label: "公开订阅", value: dashboard?.outputCount ?? 0, sub: `${dashboard?.enabledOutputs ?? 0} 个启用` },
    { label: "订阅源", value: dashboard?.sourceCount ?? 0, sub: `${dashboard?.enabledSources ?? 0} 个启用` },
    { label: "正常源", value: dashboard?.healthySources ?? 0, sub: `${dashboard?.unhealthySources ?? 0} 个异常` },
    { label: "缓存节点", value: dashboard?.totalCachedNodes ?? 0, sub: formatTime(dashboard?.lastRefreshAt) },
  ]
  return (
    <div className="flex flex-col gap-4">
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        {stats.map((stat) => (
          <Card key={stat.label}>
            <CardHeader className="pb-2">
              <CardDescription>{stat.label}</CardDescription>
              <CardTitle className="text-2xl">{loading ? <Skeleton className="h-8 w-16" /> : stat.value}</CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-sm text-muted-foreground">{stat.sub}</p>
            </CardContent>
          </Card>
        ))}
      </div>
      <Card>
        <CardHeader>
          <CardTitle>稳定订阅地址</CardTitle>
          <CardDescription>客户端只需要订阅固定地址，后续上游变化在后台维护。</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          {outputs.length === 0 ? (
            <Alert>
              <FileJsonIcon />
              <AlertTitle>还没有公开订阅</AlertTitle>
              <AlertDescription>进入“公开订阅”创建 `main` 后即可复制地址到 Clash / Mihomo。</AlertDescription>
            </Alert>
          ) : (
            outputs.map((output) => (
              <div key={output.id} className="flex flex-col gap-2 rounded-lg border p-3 sm:flex-row sm:items-center sm:justify-between">
                <div className="min-w-0">
                  <div className="flex items-center gap-2">
                    <p className="truncate font-medium">{output.name}</p>
                    <StatusBadge status={output.enabled ? "ok" : "paused"} />
                  </div>
                  <p className="truncate text-sm text-muted-foreground">
                    {subscriptionURL(publicBase, output.slug, userToken)}
                  </p>
                </div>
                <Button variant="outline" size="sm" onClick={() => copyText(subscriptionURL(publicBase, output.slug, userToken), "订阅地址已复制")}>
                  <ClipboardIcon data-icon="inline-start" />
                  复制
                </Button>
              </div>
            ))
          )}
        </CardContent>
      </Card>
    </div>
  )
}

function SourcesView({
  api,
  sources,
  loading,
  busy,
  setBusy,
  scopeQuery,
  reload,
}: {
  api: API
  sources: Source[]
  loading: boolean
  busy: string
  setBusy: (value: string) => void
  scopeQuery: string
  reload: (options?: { silent?: boolean }) => Promise<void>
}) {
  const [editing, setEditing] = useState<Source | null>(null)
  const [expanded, setExpanded] = useState<Record<string, boolean>>({})

  function openSourceSheet(source: Source) {
    setEditing(source)
    toast.message(source.id ? `正在编辑「${source.name}」` : "正在添加订阅源")
  }

  function toggleSourceNodes(source: Source) {
    const nextExpanded = !expanded[source.id]
    setExpanded({ ...expanded, [source.id]: nextExpanded })
    toast.message(nextExpanded ? `已展开「${source.name}」节点` : `已收起「${source.name}」节点`)
  }

  async function refreshSource(source: Source) {
    setBusy(`refresh:${source.id}`)
    try {
      await api.post(withScope(`/api/sources/${source.id}/refresh`, scopeQuery), {})
      toast.success(`${source.name} 已开始刷新`)
      await reload({ silent: true })
    } catch (error) {
      toast.error(messageOf(error))
      await reload({ silent: true })
    } finally {
      setBusy("")
    }
  }

  useEffect(() => {
    if (!sources.some((source) => source.lastStatus === "refreshing")) {
      return
    }
    const timer = window.setInterval(() => {
      void reload({ silent: true })
    }, 2000)
    return () => window.clearInterval(timer)
  }, [reload, sources])

  async function deleteSource(source: Source) {
    if (!confirm(`删除订阅源「${source.name}」？`)) {
      return
    }
    try {
      await api.delete(withScope(`/api/sources/${source.id}`, scopeQuery))
      toast.success("订阅源已删除")
      await reload()
    } catch (error) {
      toast.error(messageOf(error))
    }
  }

  return (
    <Card>
      <CardHeader className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <CardTitle>订阅源</CardTitle>
          <CardDescription>添加多个上游订阅，失败时会继续使用上一次成功缓存。</CardDescription>
        </div>
        <Button onClick={() => openSourceSheet(defaultSource)}>
          <PlusIcon data-icon="inline-start" />
          添加订阅源
        </Button>
      </CardHeader>
      <CardContent>
        {loading ? (
          <TableSkeleton />
        ) : sources.length === 0 ? (
          <Empty className="min-h-64">
            <EmptyHeader>
              <EmptyMedia variant="icon"><ServerIcon /></EmptyMedia>
              <EmptyTitle>还没有订阅源</EmptyTitle>
              <EmptyDescription>添加一个 base64、Clash 或明文分享链接订阅源开始聚合。</EmptyDescription>
            </EmptyHeader>
            <EmptyContent>
              <Button onClick={() => openSourceSheet(defaultSource)}>
                <PlusIcon data-icon="inline-start" />
                添加订阅源
              </Button>
            </EmptyContent>
          </Empty>
        ) : (
          <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>名称</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead>格式</TableHead>
                  <TableHead>节点</TableHead>
                  <TableHead>订阅链接</TableHead>
                  <TableHead className="text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {sources.map((source) => (
                  <Fragment key={source.id}>
                  <TableRow key={source.id}>
                    <TableCell className="min-w-44">
                      <div className="flex flex-col gap-1">
                        <div className="flex items-center gap-2">
                          <Button
                            variant="ghost"
                            size="icon-sm"
                            aria-label={expanded[source.id] ? "收起节点" : "展开节点"}
                            aria-expanded={expanded[source.id] ? "true" : "false"}
                            onClick={() => toggleSourceNodes(source)}
                          >
                            {expanded[source.id] ? <FiChevronDown /> : <FiChevronRight />}
                            <span className="sr-only">{expanded[source.id] ? "收起节点" : "展开节点"}</span>
                          </Button>
                          <span className="font-medium">{source.name}</span>
                        </div>
                        <span className="text-xs text-muted-foreground">{source.remark || "无备注"}</span>
                      </div>
                    </TableCell>
                    <TableCell className="min-w-56">
                      <div className="flex min-w-0 flex-col gap-1">
                        <div className="flex min-w-0 items-center gap-2">
                          <StatusBadge status={source.enabled ? source.lastStatus : "paused"} />
                          {source.refreshProgress ? (
                            <span className="max-w-56 truncate text-xs text-muted-foreground">{source.refreshProgress}</span>
                          ) : source.lastError ? (
                            <span className="max-w-40 truncate text-xs text-muted-foreground">{source.lastError}</span>
                          ) : null}
                        </div>
                        {source.lastStatus === "refreshing" ? (
                          <ProgressBar value={source.refreshPercent || 1} />
                        ) : null}
                      </div>
                    </TableCell>
                    <TableCell>{source.lastFormat || "-"}</TableCell>
                    <TableCell className="min-w-56">
                      <div className="flex w-max items-center gap-1.5 whitespace-nowrap">
                        <Badge variant="outline" className="font-mono tabular-nums">{source.lastNodeCount}</Badge>
                        <Badge variant="secondary">可用 {source.nodeStats?.alive ?? 0}</Badge>
                        <Badge variant={(source.nodeStats?.dead ?? 0) > 0 ? "destructive" : "outline"}>失败 {source.nodeStats?.dead ?? 0}</Badge>
                        <Badge variant="outline">未测 {source.nodeStats?.unchecked ?? 0}</Badge>
                      </div>
                    </TableCell>
                    <TableCell className="max-w-64 truncate text-muted-foreground">{source.urlMasked}</TableCell>
                    <TableCell>
                      <div className="flex justify-end gap-2">
                        <Button variant="outline" size="sm" onClick={() => refreshSource(source)} disabled={busy === `refresh:${source.id}` || source.lastStatus === "refreshing"}>
                          {busy === `refresh:${source.id}` || source.lastStatus === "refreshing" ? <Loader2Icon data-icon="inline-start" /> : <RefreshCcwIcon data-icon="inline-start" />}
                          {source.lastStatus === "refreshing" ? "刷新中" : "刷新"}
                        </Button>
                        <Button variant="outline" size="sm" onClick={() => openSourceSheet(source)}>编辑</Button>
                        <Button variant="ghost" size="icon-sm" onClick={() => deleteSource(source)}>
                          <Trash2Icon />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                  {expanded[source.id] ? (
                    <TableRow key={`${source.id}-nodes`}>
                      <TableCell colSpan={6}>
                        <SourceNodesTable nodes={source.nodes ?? []} />
                      </TableCell>
                    </TableRow>
                  ) : null}
                  </Fragment>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </CardContent>
      <SourceSheet
        source={editing}
        onOpenChange={(open) => !open && setEditing(null)}
        onSave={async (source) => {
          try {
            let saved: Source
            if (source.id) {
              saved = await api.put<Source>(withScope(`/api/sources/${source.id}`, scopeQuery), source)
            } else {
              saved = await api.post<Source>(withScope("/api/sources", scopeQuery), source)
            }
            toast.success("订阅源已保存")
            if ((saved.sourceType ?? "url") === "file") {
              await api.post(withScope(`/api/sources/${saved.id}/refresh`, scopeQuery), {})
              toast.success("文件订阅已开始解析")
            }
            setEditing(null)
            await reload()
          } catch (error) {
            toast.error(messageOf(error))
          }
        }}
      />
    </Card>
  )
}

function SourceNodesTable({ nodes }: { nodes: NonNullable<Source["nodes"]> }) {
  if (nodes.length === 0) {
    return (
      <Empty className="min-h-32">
        <EmptyHeader>
          <EmptyTitle>暂无节点缓存</EmptyTitle>
          <EmptyDescription>刷新订阅源后会显示原始节点和检测状态。</EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  }
  return (
    <div className="rounded-lg border bg-muted/20 p-2">
      <div className="overflow-x-auto">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>原始节点</TableHead>
              <TableHead>地址</TableHead>
              <TableHead>地区</TableHead>
              <TableHead>状态</TableHead>
              <TableHead>出口 / 解析 IP</TableHead>
              <TableHead>失败原因</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {nodes.map((node) => (
              <TableRow key={`${node.key}-${node.server}-${node.port}`}>
                <TableCell className="max-w-64 truncate">{node.originalName || node.name}</TableCell>
                <TableCell className="max-w-64 truncate text-muted-foreground">{node.server}:{node.port}</TableCell>
                <TableCell>
                  <Badge variant={node.region === "其他节点" ? "outline" : "secondary"}>
                    {node.regionCode || "OTHER"}
                  </Badge>
                </TableCell>
                <TableCell>
                  <Badge variant={node.alive === false ? "destructive" : node.alive === true ? "secondary" : "outline"}>
                    {node.alive === false ? "不可用" : node.alive === true ? "可用" : "未检测"}
                  </Badge>
                </TableCell>
                <TableCell className="text-muted-foreground">{node.exitIp || node.resolvedIp || "-"}</TableCell>
                <TableCell className="max-w-72 truncate text-muted-foreground">{node.probeError || node.excludedReason || "-"}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}

function ProgressBar({ value }: { value: number }) {
  const percent = Math.max(1, Math.min(100, Math.round(value)))
  return (
    <div className="h-1.5 w-full overflow-hidden rounded-full bg-muted" aria-label={`刷新进度 ${percent}%`}>
      <div className="h-full rounded-full bg-primary transition-all" style={{ width: `${percent}%` }} />
    </div>
  )
}

function SourceSheet({
  source,
  onOpenChange,
  onSave,
}: {
  source: Source | null
  onOpenChange: (open: boolean) => void
  onSave: (source: Source) => Promise<void>
}) {
  const [draft, setDraft] = useState<Source>(defaultSource)
  const open = Boolean(source)
  const sourceType = draft.sourceType ?? "url"

  useEffect(() => {
    setDraft(source ?? defaultSource)
  }, [source])

  async function loadSourceFile(file?: File) {
    if (!file) {
      return
    }
    const content = await file.text()
    setDraft((current) => ({
      ...current,
      sourceType: "file",
      url: "",
      fileName: file.name,
      fileContent: content,
      name: current.name || file.name.replace(/\.[^.]+$/, ""),
    }))
    toast.success(`已读取文件「${file.name}」`)
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="w-full overflow-y-auto sm:max-w-lg">
        <SheetHeader>
          <SheetTitle>{draft.id ? "编辑订阅源" : "添加订阅源"}</SheetTitle>
          <SheetDescription>订阅链接或文件仅用于后台解析，列表中默认展示脱敏地址或文件名。</SheetDescription>
        </SheetHeader>
        <div className="px-4">
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="source-name">名称</FieldLabel>
              <Input id="source-name" value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} />
            </Field>
            <Field>
              <FieldLabel>来源类型</FieldLabel>
              <div className="flex flex-wrap gap-2">
                <Button
                  type="button"
                  variant={sourceType === "url" ? "default" : "outline"}
                  size="sm"
                  onClick={() => setDraft({ ...draft, sourceType: "url", fileName: "", fileContent: "" })}
                >
                  订阅链接
                </Button>
                <Button
                  type="button"
                  variant={sourceType === "file" ? "default" : "outline"}
                  size="sm"
                  onClick={() => setDraft({ ...draft, sourceType: "file", url: "" })}
                >
                  上传文件
                </Button>
              </div>
            </Field>
            <Field>
              <FieldLabel htmlFor={sourceType === "file" ? "source-file" : "source-url"}>
                {sourceType === "file" ? "订阅文件" : "订阅链接"}
              </FieldLabel>
              {sourceType === "file" ? (
                <div className="flex flex-col gap-2">
                  <Input
                    id="source-file"
                    type="file"
                    accept=".yaml,.yml,.txt,.conf,.list,.json,text/plain,application/x-yaml,application/yaml"
                    onChange={(event) => void loadSourceFile(event.target.files?.[0])}
                  />
                  <FieldDescription>
                    {draft.fileName ? `已选择：${draft.fileName}` : "支持 Clash/Mihomo YAML、base64 或明文分享链接文件。"}
                  </FieldDescription>
                </div>
              ) : (
                <Textarea id="source-url" className="min-h-24" value={draft.url ?? ""} onChange={(event) => setDraft({ ...draft, sourceType: "url", url: event.target.value })} />
              )}
            </Field>
            <Field>
              <FieldLabel htmlFor="source-tags">标签</FieldLabel>
              <Input id="source-tags" value={draft.tags.join(", ")} onChange={(event) => setDraft({ ...draft, tags: splitList(event.target.value) })} placeholder="机场A, 高速" />
            </Field>
            <Field>
              <FieldLabel htmlFor="source-remark">备注</FieldLabel>
              <Textarea id="source-remark" value={draft.remark} onChange={(event) => setDraft({ ...draft, remark: event.target.value })} />
            </Field>
            <Field orientation="horizontal">
              <div>
                <FieldLabel>启用订阅源</FieldLabel>
                <FieldDescription>停用后不会参与任何公开订阅输出。</FieldDescription>
              </div>
              <Switch checked={draft.enabled} onCheckedChange={(checked) => setDraft({ ...draft, enabled: checked })} />
            </Field>
          </FieldGroup>
        </div>
        <SheetFooter>
          <Button disabled={!draft.name || (sourceType === "file" ? !draft.fileContent : !draft.url)} onClick={() => onSave(draft)}>
            <SaveIcon data-icon="inline-start" />
            保存
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}

function OutputsView({
  api,
  outputs,
  sources,
  loading,
  scopeQuery,
  reload,
  publicBase,
  userToken,
}: {
  api: API
  outputs: Output[]
  sources: Source[]
  loading: boolean
  scopeQuery: string
  reload: () => Promise<void>
  publicBase: string
  userToken: string
}) {
  const [editing, setEditing] = useState<Output | null>(null)
  const preparedDefault = { ...defaultOutput, sourceIds: sources.map((source) => source.id) }

  function openOutputSheet(output: Output) {
    setEditing(output)
    toast.message(output.id ? `正在编辑「${output.name}」` : "正在创建公开订阅")
  }

  async function deleteOutput(output: Output) {
    if (!confirm(`删除公开订阅「${output.name}」？`)) {
      return
    }
    try {
      await api.delete(withScope(`/api/outputs/${output.id}`, scopeQuery))
      toast.success("公开订阅已删除")
      await reload()
    } catch (error) {
      toast.error(messageOf(error))
    }
  }

  return (
    <Card>
      <CardHeader className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <CardTitle>公开订阅</CardTitle>
          <CardDescription>创建一个或多个稳定输出地址，分别选择订阅源与整理规则。</CardDescription>
        </div>
        <Button onClick={() => openOutputSheet(preparedDefault)}>
          <PlusIcon data-icon="inline-start" />
          创建公开订阅
        </Button>
      </CardHeader>
      <CardContent>
        {loading ? (
          <TableSkeleton />
        ) : outputs.length === 0 ? (
          <Empty className="min-h-64">
            <EmptyHeader>
              <EmptyMedia variant="icon"><ActivityIcon /></EmptyMedia>
              <EmptyTitle>还没有公开订阅</EmptyTitle>
              <EmptyDescription>建议先创建 slug 为 `main` 的默认输出地址。</EmptyDescription>
            </EmptyHeader>
          </Empty>
        ) : (
          <div className="grid gap-3 lg:grid-cols-2">
            {outputs.map((output) => {
              const url = subscriptionURL(publicBase, output.slug, userToken)
              return (
                <Card key={output.id}>
                  <CardHeader>
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0">
                        <CardTitle className="truncate">{output.name}</CardTitle>
                        <CardDescription className="truncate">{url}</CardDescription>
                      </div>
                      <StatusBadge status={output.enabled ? "ok" : "paused"} />
                    </div>
                  </CardHeader>
                  <CardContent className="flex flex-col gap-3">
                    <div className="grid grid-cols-3 gap-2 text-sm">
                      <Metric label="格式" value={output.format} />
                      <Metric label="输入源" value={output.sourceIds.length} />
                      <Metric label="节点" value={output.lastNodeCount || "-"} />
                    </div>
                    <div className="flex flex-wrap gap-2">
                      <Button variant="outline" size="sm" onClick={() => copyText(url, "公开订阅地址已复制")}>
                        <ClipboardIcon data-icon="inline-start" />
                        复制订阅链接
                      </Button>
                      <DropdownMenu>
                        <DropdownMenuTrigger render={<Button variant="outline" size="sm" />}>
                          <DownloadIcon data-icon="inline-start" />
                          下载配置
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="start" className="w-52">
                          <DropdownMenuGroup>
                            {downloadFormatItems.map((item) => (
                              <DropdownMenuItem
                                key={item.value}
                                onClick={() => downloadSubscription(url, output.slug, item.value)}
                              >
                                <DownloadIcon />
                                {item.label}
                              </DropdownMenuItem>
                            ))}
                          </DropdownMenuGroup>
                        </DropdownMenuContent>
                      </DropdownMenu>
                      <Button variant="outline" size="sm" onClick={() => openOutputSheet(output)}>编辑</Button>
                      <Button variant="ghost" size="icon-sm" onClick={() => deleteOutput(output)}>
                        <Trash2Icon />
                      </Button>
                    </div>
                  </CardContent>
                </Card>
              )
            })}
          </div>
        )}
      </CardContent>
      <OutputSheet
        output={editing}
        sources={sources}
        onOpenChange={(open) => !open && setEditing(null)}
        onSave={async (output) => {
          try {
            if (output.id) {
              await api.put(withScope(`/api/outputs/${output.id}`, scopeQuery), output)
            } else {
              await api.post(withScope("/api/outputs", scopeQuery), output)
            }
            toast.success("公开订阅已保存")
            setEditing(null)
            await reload()
          } catch (error) {
            toast.error(messageOf(error))
          }
        }}
      />
    </Card>
  )
}

function OutputSheet({
  output,
  sources,
  onOpenChange,
  onSave,
}: {
  output: Output | null
  sources: Source[]
  onOpenChange: (open: boolean) => void
  onSave: (output: Output) => Promise<void>
}) {
  const [draft, setDraft] = useState<Output>(defaultOutput)
  const open = Boolean(output)

  useEffect(() => {
    setDraft(output ?? defaultOutput)
  }, [output])

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="w-full overflow-y-auto sm:max-w-xl">
        <SheetHeader>
          <SheetTitle>{draft.id ? "编辑公开订阅" : "创建公开订阅"}</SheetTitle>
          <SheetDescription>过滤、重命名和自动分组会在输出时统一应用。</SheetDescription>
        </SheetHeader>
        <div className="px-4">
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="output-name">名称</FieldLabel>
              <Input id="output-name" value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} />
            </Field>
            <Field>
              <FieldLabel htmlFor="output-slug">公开 slug</FieldLabel>
              <Input id="output-slug" value={draft.slug} onChange={(event) => setDraft({ ...draft, slug: event.target.value })} />
              <FieldDescription>最终地址形如 `/s/main`。</FieldDescription>
            </Field>
            <Field>
              <FieldLabel>输出格式</FieldLabel>
              <Select
                items={outputFormatItems}
                value={draft.format}
                onValueChange={(value) => setDraft({ ...draft, format: String(value) })}
              >
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectGroup>
                    {outputFormatItems.map((item) => (
                      <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>
                    ))}
                  </SelectGroup>
                </SelectContent>
              </Select>
            </Field>
            <Field>
              <FieldLabel>输入订阅源</FieldLabel>
              <div className="flex flex-col gap-2 rounded-lg border p-3">
                {sources.length === 0 ? (
                  <p className="text-sm text-muted-foreground">还没有可选择的订阅源。</p>
                ) : (
                  sources.map((source) => (
                    <label key={source.id} className="flex items-center gap-2 text-sm">
                      <Checkbox
                        checked={draft.sourceIds.includes(source.id)}
                        onCheckedChange={(checked) => {
                          setDraft({
                            ...draft,
                            sourceIds: checked
                              ? [...draft.sourceIds, source.id]
                              : draft.sourceIds.filter((id) => id !== source.id),
                          })
                        }}
                      />
                      <span className="truncate">{source.name}</span>
                      <Badge variant="outline">{source.lastNodeCount}</Badge>
                    </label>
                  ))
                )}
              </div>
            </Field>
            <Field>
              <FieldLabel htmlFor="include-keywords">只保留关键词</FieldLabel>
              <Input id="include-keywords" value={draft.filter.includeKeywords.join(", ")} onChange={(event) => setDraft({ ...draft, filter: { ...draft.filter, includeKeywords: splitList(event.target.value) } })} />
            </Field>
            <Field>
              <FieldLabel htmlFor="exclude-keywords">排除关键词</FieldLabel>
              <Input id="exclude-keywords" value={draft.filter.excludeKeywords.join(", ")} onChange={(event) => setDraft({ ...draft, filter: { ...draft.filter, excludeKeywords: splitList(event.target.value) } })} />
            </Field>
            <Field>
              <FieldLabel htmlFor="regex">正则匹配</FieldLabel>
              <Input id="regex" value={draft.filter.regex} onChange={(event) => setDraft({ ...draft, filter: { ...draft.filter, regex: event.target.value } })} />
            </Field>
            <Field>
              <FieldLabel htmlFor="rename-rules">重命名规则</FieldLabel>
              <Textarea
                id="rename-rules"
                className="min-h-24 font-mono text-xs"
                value={draft.renameRules.map((rule) => `${rule.pattern} => ${rule.replacement}`).join("\n")}
                onChange={(event) => setDraft({ ...draft, renameRules: parseRenameRules(event.target.value) })}
                placeholder="香港 IEPL (\\d+) => [JMS] 香港 $1"
              />
            </Field>
            <Field orientation="horizontal">
              <div>
                <FieldLabel>启用公开地址</FieldLabel>
                <FieldDescription>暂停后 `/s/{draft.slug || "slug"}` 会拒绝访问。</FieldDescription>
              </div>
              <Switch checked={draft.enabled} onCheckedChange={(checked) => setDraft({ ...draft, enabled: checked })} />
            </Field>
          </FieldGroup>
        </div>
        <SheetFooter>
          <Button disabled={!draft.name || !draft.slug} onClick={() => onSave(draft)}>
            <SaveIcon data-icon="inline-start" />
            保存
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}

function PreviewView({
  api,
  outputs,
  preview,
  setPreview,
  scopeQuery,
}: {
  api: API
  outputs: Output[]
  preview: Preview | null
  setPreview: (preview: Preview | null) => void
  scopeQuery: string
}) {
  const [selected, setSelected] = useState(outputs[0]?.id ?? "")
  const [loading, setLoading] = useState(false)
  const [nameDrafts, setNameDrafts] = useState<Record<string, string>>({})

  useEffect(() => {
    if (!selected && outputs[0]?.id) {
      setSelected(outputs[0].id)
    }
  }, [outputs, selected])

  useEffect(() => {
    if (!preview) {
      return
    }
    setNameDrafts(Object.fromEntries(preview.nodes.map((node) => [node.key, node.name])))
  }, [preview])

  async function loadPreview(id = selected) {
    if (!id) {
      return
    }
    setLoading(true)
    try {
      setPreview(await api.get<Preview>(withScope(`/api/outputs/${id}/preview`, scopeQuery)))
      toast.success("预览已生成")
    } catch (error) {
      toast.error(messageOf(error))
    } finally {
      setLoading(false)
    }
  }

  async function saveNodeNames() {
    const output = outputs.find((item) => item.id === selected)
    if (!output) {
      return
    }
    const cleaned = Object.fromEntries(
      Object.entries(nameDrafts).map(([key, value]) => [key, value.trim()]).filter(([, value]) => value),
    )
    try {
      await api.put(withScope(`/api/outputs/${output.id}`, scopeQuery), {
        ...output,
        nodeNameOverrides: cleaned,
      })
      toast.success("节点名称已保存")
      await loadPreview(output.id)
    } catch (error) {
      toast.error(messageOf(error))
    }
  }

  if (outputs.length === 0) {
    return (
      <Empty className="min-h-72 border">
        <EmptyHeader>
          <EmptyMedia variant="icon"><EyeIcon /></EmptyMedia>
          <EmptyTitle>暂无可预览的公开订阅</EmptyTitle>
          <EmptyDescription>创建公开订阅后，这里会展示去重、过滤和地区分组结果。</EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  }

  const selectItems = outputs.map((output) => ({ label: output.name, value: output.id }))

  return (
    <div className="flex flex-col gap-4">
      <Card>
        <CardHeader className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <CardTitle>输出预览</CardTitle>
            <CardDescription>检查最终节点数量、被过滤节点和地区分组。</CardDescription>
          </div>
          <div className="flex gap-2">
            <Select items={selectItems} value={selected} onValueChange={(value) => setSelected(String(value))}>
              <SelectTrigger className="w-52">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  {selectItems.map((item) => (
                    <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
            <Button variant="outline" onClick={() => loadPreview()} disabled={loading}>
              {loading ? <Loader2Icon data-icon="inline-start" /> : <RefreshCcwIcon data-icon="inline-start" />}
              生成预览
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          {!preview ? (
            <Empty className="min-h-48">
              <EmptyHeader>
                <EmptyTitle>选择一个公开订阅生成预览</EmptyTitle>
              </EmptyHeader>
            </Empty>
          ) : (
            <div className="flex flex-col gap-4">
              <div className="grid gap-3 sm:grid-cols-4">
                <Metric label="输出节点" value={preview.nodeCount} />
                <Metric label="重复移除" value={preview.duplicateCount} />
                <Metric label="不可用移除" value={preview.unavailableCount} />
                <Metric label="使用缓存源" value={preview.usedCachedSources} />
              </div>
              <Separator />
              <div className="grid gap-3 lg:grid-cols-2">
                {preview.groups.map((group) => (
                  <Card key={group.name}>
                    <CardHeader className="pb-2">
                      <CardTitle className="text-base">{group.name}</CardTitle>
                      <CardDescription>{group.nodes.length} 个节点</CardDescription>
                    </CardHeader>
                    <CardContent>
                      <div className="flex max-h-40 flex-col gap-1 overflow-y-auto text-sm text-muted-foreground">
                        {group.nodes.slice(0, 40).map((node) => <span key={node} className="truncate">{node}</span>)}
                        {group.nodes.length > 40 ? <span>还有 {group.nodes.length - 40} 个...</span> : null}
                      </div>
                    </CardContent>
                  </Card>
                ))}
              </div>
              <Card>
                <CardHeader>
                  <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                    <div>
                      <CardTitle className="text-base">节点名称与地址识别</CardTitle>
                      <CardDescription>默认按地区编号命名；可为单个节点覆盖输出名称。</CardDescription>
                    </div>
                    <Button variant="outline" size="sm" onClick={saveNodeNames}>
                      <SaveIcon data-icon="inline-start" />
                      保存名称
                    </Button>
                  </div>
                </CardHeader>
                <CardContent>
                  <div className="overflow-x-auto">
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>输出名称</TableHead>
                          <TableHead>原始名称</TableHead>
                          <TableHead>地址</TableHead>
                          <TableHead>地区</TableHead>
                          <TableHead>可用性</TableHead>
                          <TableHead>出口 / 解析 IP</TableHead>
                          <TableHead>来源</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {preview.nodes.slice(0, 80).map((node) => (
                          <TableRow key={`${node.name}-${node.server}-${node.port}`}>
                            <TableCell className="min-w-44">
                              <Input
                                value={nameDrafts[node.key] ?? node.name}
                                onChange={(event) => setNameDrafts({ ...nameDrafts, [node.key]: event.target.value })}
                              />
                            </TableCell>
                            <TableCell className="max-w-64 truncate text-muted-foreground">{node.originalName || node.name}</TableCell>
                            <TableCell className="max-w-72 truncate text-muted-foreground">{node.server}:{node.port}</TableCell>
                            <TableCell>
                              <Badge variant={node.region === "其他节点" ? "outline" : "secondary"}>
                                {node.regionCode || "OTHER"}
                              </Badge>
                            </TableCell>
                            <TableCell>
                              <Badge variant={node.alive === false ? "destructive" : node.alive === true ? "secondary" : "outline"}>
                                {node.alive === false ? "不可用" : node.alive === true ? "可用" : "未检测"}
                              </Badge>
                            </TableCell>
                            <TableCell className="text-muted-foreground">{node.exitIp || node.resolvedIp || "-"}</TableCell>
                            <TableCell className="max-w-48 truncate text-muted-foreground">
                              {node.probeStatus === "ok" ? "真实出口" : `兜底${node.probeError ? `：${node.probeError}` : ""}`}
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </div>
                  {preview.nodes.length > 80 ? (
                    <p className="mt-3 text-sm text-muted-foreground">仅展示前 80 个节点。</p>
                  ) : null}
                </CardContent>
              </Card>
              {preview.excludedNodes.length > 0 ? (
                <Card>
                  <CardHeader>
                    <CardTitle className="text-base">已排除不可用节点</CardTitle>
                    <CardDescription>这些节点不会进入公开订阅输出。</CardDescription>
                  </CardHeader>
                  <CardContent>
                    <div className="flex flex-col gap-2 text-sm">
                      {preview.excludedNodes.map((node) => (
                        <div key={node.key} className="flex flex-col gap-1 rounded-lg border p-3 sm:flex-row sm:items-center sm:justify-between">
                          <span className="truncate font-medium">{node.originalName || node.name}</span>
                          <span className="truncate text-muted-foreground">{node.excludedReason || node.probeError || "不可用"}</span>
                        </div>
                      ))}
                    </div>
                  </CardContent>
                </Card>
              ) : null}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

function BackupView({ api, reload }: { api: API; reload: () => Promise<void> }) {
  const [backupText, setBackupText] = useState("")

  async function downloadBackup() {
    try {
      const response = await fetch("/api/backup", {
        headers: { Authorization: `Bearer ${api.token}` },
      })
      if (!response.ok) {
        throw new Error("备份下载失败")
      }
      const blob = await response.blob()
      const url = URL.createObjectURL(blob)
      const link = document.createElement("a")
      link.href = url
      link.download = "sub-nest-backup.json"
      link.click()
      URL.revokeObjectURL(url)
      toast.success("备份已开始下载")
    } catch (error) {
      toast.error(messageOf(error))
    }
  }

  async function restore() {
    try {
      const parsed = JSON.parse(backupText)
      await api.post("/api/restore", parsed)
      toast.success("配置已恢复")
      setBackupText("")
      await reload()
    } catch (error) {
      toast.error(messageOf(error))
    }
  }

  return (
    <div className="grid gap-4 lg:grid-cols-2">
      <Card>
        <CardHeader>
          <CardTitle>导出备份</CardTitle>
          <CardDescription>导出完整配置、订阅源状态和缓存节点，用于迁移或恢复。</CardDescription>
        </CardHeader>
        <CardContent>
          <Alert>
            <DatabaseBackupIcon />
            <AlertTitle>备份文件包含完整订阅链接</AlertTitle>
            <AlertDescription>请把文件保存在私密位置，不要发到公开仓库或聊天群。</AlertDescription>
          </Alert>
        </CardContent>
        <CardFooter>
          <Button onClick={downloadBackup}>
            <DatabaseBackupIcon data-icon="inline-start" />
            下载备份
          </Button>
        </CardFooter>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>导入恢复</CardTitle>
          <CardDescription>粘贴备份 JSON 后恢复当前配置。</CardDescription>
        </CardHeader>
        <CardContent>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="backup-json">备份 JSON</FieldLabel>
              <Textarea id="backup-json" className="min-h-48 font-mono text-xs" value={backupText} onChange={(event) => setBackupText(event.target.value)} />
            </Field>
          </FieldGroup>
        </CardContent>
        <CardFooter>
          <Button variant="outline" disabled={!backupText.trim()} onClick={restore}>
            <ArchiveRestoreIcon data-icon="inline-start" />
            恢复配置
          </Button>
        </CardFooter>
      </Card>
    </div>
  )
}

function UsersView({ api, users, reload }: { api: API; users: User[]; reload: () => Promise<void> }) {
  const [inviteCodes, setInviteCodes] = useState<InviteCode[]>([])
  const [label, setLabel] = useState("")
  const [createdCode, setCreatedCode] = useState("")
  const [loading, setLoading] = useState(false)

  const loadInvites = useCallback(async () => {
    try {
      setInviteCodes(await api.get<InviteCode[]>("/api/admin/invite-codes"))
    } catch (error) {
      toast.error(messageOf(error))
    }
  }, [api])

  useEffect(() => {
    void loadInvites()
  }, [loadInvites])

  async function createInvite() {
    setLoading(true)
    try {
      const invite = await api.post<CreatedInviteCode>("/api/admin/invite-codes", { label })
      setCreatedCode(invite.code)
      setLabel("")
      toast.success("授权码已创建")
      await loadInvites()
    } catch (error) {
      toast.error(messageOf(error))
    } finally {
      setLoading(false)
    }
  }

  async function setUserEnabled(user: User, enabled: boolean) {
    try {
      await api.put(`/api/admin/users/${user.id}`, { enabled })
      toast.success(enabled ? "用户已启用" : "用户已禁用")
      await reload()
    } catch (error) {
      toast.error(messageOf(error))
    }
  }

  const userName = (id?: string) => users.find((item) => item.id === id)?.name || users.find((item) => item.id === id)?.slug || id || "-"

  return (
    <div className="grid gap-4 xl:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]">
      <Card>
        <CardHeader>
          <CardTitle>授权码</CardTitle>
          <CardDescription>授权码只显示一次，用户注册成功后自动失效。</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="invite-label">备注</FieldLabel>
              <Input id="invite-label" value={label} onChange={(event) => setLabel(event.target.value)} placeholder="例如 Alice" />
            </Field>
            <Button onClick={createInvite} disabled={loading}>
              {loading ? <Loader2Icon data-icon="inline-start" /> : <KeyRoundIcon data-icon="inline-start" />}
              创建授权码
            </Button>
          </FieldGroup>
          {createdCode ? (
            <Alert>
              <KeyRoundIcon />
              <AlertTitle>请现在复制授权码</AlertTitle>
              <AlertDescription className="flex flex-col gap-2">
                <span className="break-all font-mono text-sm">{createdCode}</span>
                <Button variant="outline" size="sm" className="w-fit" onClick={() => copyText(createdCode, "授权码已复制")}>
                  <ClipboardIcon data-icon="inline-start" />
                  复制授权码
                </Button>
              </AlertDescription>
            </Alert>
          ) : null}
          <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>备注</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead>使用者</TableHead>
                  <TableHead>创建时间</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {inviteCodes.length === 0 ? (
                  <TableRow><TableCell colSpan={4} className="text-muted-foreground">暂无授权码</TableCell></TableRow>
                ) : inviteCodes.map((invite) => (
                  <TableRow key={invite.id}>
                    <TableCell className="max-w-40 truncate">{invite.label || "-"}</TableCell>
                    <TableCell><Badge variant={invite.usedAt ? "secondary" : "outline"}>{invite.usedAt ? "已使用" : "未使用"}</Badge></TableCell>
                    <TableCell className="max-w-40 truncate">{userName(invite.usedByUserId)}</TableCell>
                    <TableCell className="text-muted-foreground">{formatTime(invite.createdAt)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>用户</CardTitle>
          <CardDescription>admin 可切换到任意用户空间进行管理。</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>用户</TableHead>
                  <TableHead>角色</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead>最近登录</TableHead>
                  <TableHead className="text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {users.map((item) => (
                  <TableRow key={item.id}>
                    <TableCell className="min-w-48">
                      <div className="flex flex-col">
                        <span className="font-medium">{item.name || item.slug}</span>
                        <span className="text-xs text-muted-foreground">/{item.slug}</span>
                      </div>
                    </TableCell>
                    <TableCell><Badge variant={item.role === "admin" ? "default" : "outline"}>{item.role}</Badge></TableCell>
                    <TableCell><Badge variant={item.enabled ? "secondary" : "destructive"}>{item.enabled ? "启用" : "禁用"}</Badge></TableCell>
                    <TableCell className="text-muted-foreground">{formatTime(item.lastLoginAt)}</TableCell>
                    <TableCell className="text-right">
                      {item.role === "admin" ? (
                        <Button variant="ghost" size="sm" disabled>固定启用</Button>
                      ) : (
                        <Button variant="outline" size="sm" onClick={() => setUserEnabled(item, !item.enabled)}>
                          {item.enabled ? "禁用" : "启用"}
                        </Button>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}

function SettingsView({
  api,
  settings,
  userToken,
  onAdminTokenUpdated,
  onUserTokenUpdated,
}: {
  api: API
  settings: Settings | null
  userToken: string
  onAdminTokenUpdated: (token: string) => void
  onUserTokenUpdated: (settings: Settings, userToken: string) => void
}) {
  const [currentAdminToken, setCurrentAdminToken] = useState("")
  const [newAdminToken, setNewAdminToken] = useState("")
  const [confirmAdminToken, setConfirmAdminToken] = useState("")
  const [userTokenDraft, setUserTokenDraft] = useState(userToken)
  const [busy, setBusy] = useState("")

  useEffect(() => {
    setUserTokenDraft(userToken)
  }, [userToken])

  async function updateAdminToken() {
    if (newAdminToken !== confirmAdminToken) {
      toast.error("两次输入的新管理员 token 不一致")
      return
    }
    setBusy("admin-token")
    try {
      const response = await api.put<{ token: string }>("/api/settings/admin-token", {
        currentToken: currentAdminToken,
        newToken: newAdminToken,
      })
      onAdminTokenUpdated(response.token)
      setCurrentAdminToken("")
      setNewAdminToken("")
      setConfirmAdminToken("")
      toast.success("管理员 token 已修改")
    } catch (error) {
      toast.error(messageOf(error))
    } finally {
      setBusy("")
    }
  }

  async function updateUserToken(nextToken = userTokenDraft.trim()) {
    setBusy("user-token")
    try {
      const nextSettings = await api.put<Settings>("/api/settings/user-token", { token: nextToken })
      onUserTokenUpdated(nextSettings, nextToken)
      setUserTokenDraft(nextToken)
      toast.success(nextToken ? "用户 token 已保存" : "用户 token 已清空")
    } catch (error) {
      toast.error(messageOf(error))
    } finally {
      setBusy("")
    }
  }

  const userTokenEnabled = settings?.hasUserToken ?? false
  const canSaveAdminToken = currentAdminToken.length >= 8 && newAdminToken.length >= 8 && confirmAdminToken.length >= 8
  const canSaveUserToken = userTokenDraft.trim().length >= 8

  return (
    <div className="grid gap-4 lg:grid-cols-2">
      <Card>
        <CardHeader>
          <div className="flex items-start justify-between gap-3">
            <div>
              <CardTitle>管理员 token</CardTitle>
              <CardDescription>用于登录后台和管理配置，修改后当前浏览器会自动换成新会话。</CardDescription>
            </div>
            <Badge variant="secondary"><ShieldIcon />后台</Badge>
          </div>
        </CardHeader>
        <CardContent>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="current-admin-token">当前管理员 token</FieldLabel>
              <Input
                id="current-admin-token"
                type="password"
                value={currentAdminToken}
                onChange={(event) => setCurrentAdminToken(event.target.value)}
                placeholder="输入当前 token"
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="new-admin-token">新管理员 token</FieldLabel>
              <Input
                id="new-admin-token"
                type="password"
                value={newAdminToken}
                onChange={(event) => setNewAdminToken(event.target.value)}
                placeholder="至少 8 位"
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="confirm-admin-token">确认新 token</FieldLabel>
              <Input
                id="confirm-admin-token"
                type="password"
                value={confirmAdminToken}
                onChange={(event) => setConfirmAdminToken(event.target.value)}
                placeholder="再次输入新 token"
              />
            </Field>
          </FieldGroup>
        </CardContent>
        <CardFooter>
          <Button disabled={busy === "admin-token" || !canSaveAdminToken} onClick={updateAdminToken}>
            {busy === "admin-token" ? <Loader2Icon data-icon="inline-start" /> : <SaveIcon data-icon="inline-start" />}
            修改管理员 token
          </Button>
        </CardFooter>
      </Card>

      <Card>
        <CardHeader>
          <div className="flex items-start justify-between gap-3">
            <div>
              <CardTitle>公开订阅 token</CardTitle>
              <CardDescription>可选，用于给公开订阅地址增加访问 token。</CardDescription>
            </div>
            <Badge variant={userTokenEnabled ? "default" : "outline"}>
              <KeyRoundIcon />
              {userTokenEnabled ? "已启用" : "未启用"}
            </Badge>
          </div>
        </CardHeader>
        <CardContent>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="user-token">公开订阅用户 token</FieldLabel>
              <Input
                id="user-token"
                type="password"
                value={userTokenDraft}
                onChange={(event) => setUserTokenDraft(event.target.value)}
                placeholder={userTokenEnabled ? "输入新的用户 token" : "至少 8 位"}
              />
              <FieldDescription>
                {userTokenEnabled
                  ? userToken
                    ? "本浏览器已保存用户 token，复制订阅地址时会自动带上。"
                    : "服务端已启用用户 token；如需复制可用链接，请在这里重新输入并保存。"
                  : "未设置时公开订阅地址保持原来的直接访问方式。"}
              </FieldDescription>
            </Field>
          </FieldGroup>
        </CardContent>
        <CardFooter className="flex flex-wrap gap-2">
          <Button disabled={busy === "user-token" || !canSaveUserToken} onClick={() => updateUserToken()}>
            {busy === "user-token" ? <Loader2Icon data-icon="inline-start" /> : <SaveIcon data-icon="inline-start" />}
            保存用户 token
          </Button>
          <Button variant="outline" disabled={busy === "user-token" || !userTokenEnabled} onClick={() => updateUserToken("")}>
            清空用户 token
          </Button>
        </CardFooter>
      </Card>
    </div>
  )
}

function Metric({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="rounded-lg border p-3">
      <p className="text-xs text-muted-foreground">{label}</p>
      <p className="mt-1 truncate text-lg font-semibold">{value}</p>
    </div>
  )
}

function StatusBadge({ status }: { status: string }) {
  if (status === "ok") {
    return <Badge><CheckCircle2Icon />正常</Badge>
  }
  if (status === "refreshing") {
    return <Badge variant="secondary"><Loader2Icon />刷新中</Badge>
  }
  if (status === "paused") {
    return <Badge variant="outline"><PauseCircleIcon />暂停</Badge>
  }
  if (status === "error") {
    return <Badge variant="destructive"><WifiOffIcon />异常</Badge>
  }
  return <Badge variant="secondary">待刷新</Badge>
}

function LoadingShell() {
  return (
    <main className="mx-auto flex min-h-screen w-full max-w-5xl flex-col gap-4 p-6">
      <Skeleton className="h-12 w-72" />
      <div className="grid gap-3 sm:grid-cols-4">
        {Array.from({ length: 4 }).map((_, index) => <Skeleton key={index} className="h-32" />)}
      </div>
      <Skeleton className="h-96" />
    </main>
  )
}

function TableSkeleton() {
  return (
    <div className="flex flex-col gap-2">
      {Array.from({ length: 5 }).map((_, index) => <Skeleton key={index} className="h-12" />)}
    </div>
  )
}

function createAPI(token: string) {
  return {
    token,
    get<T>(path: string) {
      return request<T>(path, { method: "GET" }, token)
    },
    post<T>(path: string, body: unknown) {
      return request<T>(path, { method: "POST", body: JSON.stringify(body) }, token)
    },
    put<T>(path: string, body: unknown) {
      return request<T>(path, { method: "PUT", body: JSON.stringify(body) }, token)
    },
    delete<T>(path: string) {
      return request<T>(path, { method: "DELETE" }, token)
    },
  }
}

type API = ReturnType<typeof createAPI>

function readStoredUser(): User | null {
  const value = localStorage.getItem("sub-nest-user")
  if (!value) {
    return null
  }
  try {
    return JSON.parse(value) as User
  } catch {
    localStorage.removeItem("sub-nest-user")
    return null
  }
}

function withScope(path: string, scopeQuery: string) {
  if (!scopeQuery) {
    return path
  }
  const [base, query = ""] = path.split("?")
  const scope = scopeQuery.replace(/^\?/, "")
  return `${base}?${[query, scope].filter(Boolean).join("&")}`
}

function publicBaseFromExample(example?: string) {
  if (!example) {
    return window.location.origin
  }
  const userScoped = example.match(/^(.*\/u\/[^/]+)\/s\/[^/]+$/)
  if (userScoped) {
    return userScoped[1]
  }
  return example.replace(/\/s\/[^/]+$/, "")
}

async function request<T>(path: string, init: RequestInit, token: string): Promise<T> {
  const response = await fetch(path, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...(init.headers ?? {}),
    },
  })
  if (response.status === 204) {
    return undefined as T
  }
  const data = await response.json().catch(() => ({}))
  if (!response.ok) {
    throw new Error(data.error ?? "请求失败")
  }
  return data as T
}

function splitList(value: string) {
  return value.split(/[,，\n]/).map((item) => item.trim()).filter(Boolean)
}

function parseRenameRules(value: string) {
  return value.split("\n").map((line) => {
    const [pattern, ...rest] = line.split("=>")
    return { pattern: pattern?.trim() ?? "", replacement: rest.join("=>").trim() }
  }).filter((rule) => rule.pattern)
}

function formatTime(value?: string) {
  if (!value) {
    return "尚未刷新"
  }
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(value))
}

async function copyText(value: string, message = "已复制") {
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(value)
    } else {
      fallbackCopyText(value)
    }
    toast.success(message)
  } catch (error) {
    try {
      fallbackCopyText(value)
      toast.success(message)
    } catch {
      toast.error(messageOf(error))
    }
  }
}

function subscriptionURL(publicBase: string, slug: string, userToken = "") {
  const url = new URL(`/s/${slug}`, publicBase)
  const token = userToken.trim()
  if (token) {
    url.searchParams.set("token", token)
  }
  return url.toString()
}

function downloadSubscription(baseURL: string, slug: string, format: string) {
  const item = downloadFormatItems.find((candidate) => candidate.value === format)
  const url = new URL(baseURL, window.location.origin)
  url.searchParams.set("format", format)
  url.searchParams.set("download", "1")

  const link = document.createElement("a")
  link.href = url.toString()
  link.download = `${slug || "subscription"}-${item?.filename ?? "config.yaml"}`
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  toast.success(`${item?.label ?? "配置文件"} 已开始下载`)
}

function fallbackCopyText(value: string) {
  const textarea = document.createElement("textarea")
  textarea.value = value
  textarea.setAttribute("readonly", "")
  textarea.style.position = "fixed"
  textarea.style.top = "0"
  textarea.style.opacity = "0"
  document.body.appendChild(textarea)
  textarea.select()
  const copied = document.execCommand("copy")
  document.body.removeChild(textarea)
  if (!copied) {
    throw new Error("复制失败，请手动复制")
  }
}

function messageOf(error: unknown) {
  if (!(error instanceof Error)) {
    return "操作失败"
  }
  if (error.message === "Failed to fetch") {
    return "后端服务未运行或连接已断开，请确认 Go 服务正在监听 8080"
  }
  return error.message
}

export default App
