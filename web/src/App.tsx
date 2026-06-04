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
  Loader2Icon,
  LockIcon,
  PauseCircleIcon,
  PlusIcon,
  RefreshCcwIcon,
  SaveIcon,
  ServerIcon,
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
  const [dashboard, setDashboard] = useState<Dashboard | null>(null)
  const [sources, setSources] = useState<Source[]>([])
  const [outputs, setOutputs] = useState<Output[]>([])
  const [preview, setPreview] = useState<Preview | null>(null)
  const [activeTab, setActiveTab] = useState("overview")
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState("")

  const api = useMemo(() => createAPI(token), [token])
  const authenticated = Boolean(token)
  const anyRefreshing = sources.some((source) => source.lastStatus === "refreshing")

  const loadProtected = useCallback(async (options?: { silent?: boolean }) => {
    if (!token) {
      return
    }
    if (!options?.silent) {
      setLoading(true)
    }
    try {
      const [nextDashboard, nextSources, nextOutputs] = await Promise.all([
        api.get<Dashboard>("/api/dashboard"),
        api.get<Source[]>("/api/sources?includeUrl=1"),
        api.get<Output[]>("/api/outputs"),
      ])
      setDashboard(nextDashboard)
      setSources(nextSources)
      setOutputs(nextOutputs)
      if (nextOutputs[0]) {
        try {
          setPreview(await api.get<Preview>(`/api/outputs/${nextOutputs[0].id}/preview`))
        } catch {
          setPreview(null)
        }
      }
    } catch (error) {
      localStorage.removeItem("subagg-token")
      localStorage.removeItem("sub-nest-token")
      setToken("")
      toast.error(messageOf(error))
    } finally {
      if (!options?.silent) {
        setLoading(false)
      }
    }
  }, [api, token])

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
      const response = await createAPI("").post<{ token: string }>(
        setup ? "/api/setup" : "/api/login",
        setup ? { token: rawToken, publicBaseUrl: publicBaseURL } : { token: rawToken },
      )
      localStorage.setItem("sub-nest-token", response.token)
      localStorage.removeItem("subagg-token")
      setToken(response.token)
      setHealth({ ok: true, needsAdminSetup: false })
      toast.success(setup ? "管理员 token 已设置" : "登录成功")
    } catch (error) {
      toast.error(messageOf(error))
    } finally {
      setBusy("")
    }
  }

  async function refreshAll() {
    setBusy("refresh-all")
    try {
      await api.post("/api/refresh", {})
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
              <Button variant="outline" onClick={refreshAll} disabled={busy === "refresh-all" || anyRefreshing}>
                {busy === "refresh-all" || anyRefreshing ? <Loader2Icon data-icon="inline-start" /> : <RefreshCcwIcon data-icon="inline-start" />}
                {anyRefreshing ? "刷新中" : "刷新全部"}
              </Button>
              <Button
                variant="ghost"
                onClick={() => {
                  localStorage.removeItem("subagg-token")
                  localStorage.removeItem("sub-nest-token")
                  setToken("")
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
                <TabsTrigger value="backup">备份</TabsTrigger>
              </TabsList>
            </div>

            <TabsContent value="overview">
              <Overview dashboard={dashboard} loading={loading} outputs={outputs} />
            </TabsContent>
            <TabsContent value="sources">
              <SourcesView
                api={api}
                sources={sources}
                loading={loading}
                busy={busy}
                setBusy={setBusy}
                reload={loadProtected}
              />
            </TabsContent>
            <TabsContent value="outputs">
              <OutputsView
                api={api}
                outputs={outputs}
                sources={sources}
                loading={loading}
                reload={loadProtected}
                publicBase={dashboard?.publicExampleUrl.replace(/\/s\/main$/, "") ?? window.location.origin}
              />
            </TabsContent>
            <TabsContent value="preview">
              <PreviewView
                api={api}
                outputs={outputs}
                preview={preview}
                setPreview={setPreview}
              />
            </TabsContent>
            <TabsContent value="backup">
              <BackupView api={api} reload={loadProtected} />
            </TabsContent>
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
}: {
  setup: boolean
  busy: boolean
  onSubmit: (token: string, setup: boolean, publicBaseURL: string) => void
}) {
  const [rawToken, setRawToken] = useState("")
  const [publicBaseURL, setPublicBaseURL] = useState(window.location.origin)

  return (
    <main className="flex min-h-screen items-center justify-center bg-background p-4">
      <Card className="w-full max-w-md">
        <CardHeader>
          <div className="flex size-10 items-center justify-center rounded-lg border bg-muted">
            <LockIcon />
          </div>
          <CardTitle>{setup ? "初始化后台访问" : "登录后台"}</CardTitle>
          <CardDescription>
            后台使用本地 token 保护；订阅链接会在列表和日志中隐藏敏感部分。
          </CardDescription>
        </CardHeader>
        <CardContent>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="admin-token">管理 token</FieldLabel>
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
            disabled={busy || rawToken.length < 8}
            onClick={() => onSubmit(rawToken, setup, publicBaseURL)}
          >
            {busy ? <Loader2Icon data-icon="inline-start" /> : <ShieldIcon data-icon="inline-start" />}
            {setup ? "完成初始化" : "进入后台"}
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
}: {
  dashboard: Dashboard | null
  loading: boolean
  outputs: Output[]
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
                    {window.location.origin}/s/{output.slug}
                  </p>
                </div>
                <Button variant="outline" size="sm" onClick={() => copyText(`${window.location.origin}/s/${output.slug}`, "订阅地址已复制")}>
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
  reload,
}: {
  api: API
  sources: Source[]
  loading: boolean
  busy: string
  setBusy: (value: string) => void
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
      await api.post(`/api/sources/${source.id}/refresh`, {})
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
      await api.delete(`/api/sources/${source.id}`)
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
              saved = await api.put<Source>(`/api/sources/${source.id}`, source)
            } else {
              saved = await api.post<Source>("/api/sources", source)
            }
            toast.success("订阅源已保存")
            if ((saved.sourceType ?? "url") === "file") {
              await api.post(`/api/sources/${saved.id}/refresh`, {})
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
  reload,
  publicBase,
}: {
  api: API
  outputs: Output[]
  sources: Source[]
  loading: boolean
  reload: () => Promise<void>
  publicBase: string
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
      await api.delete(`/api/outputs/${output.id}`)
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
              const url = `${publicBase}/s/${output.slug}`
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
              await api.put(`/api/outputs/${output.id}`, output)
            } else {
              await api.post("/api/outputs", output)
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
}: {
  api: API
  outputs: Output[]
  preview: Preview | null
  setPreview: (preview: Preview | null) => void
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
      setPreview(await api.get<Preview>(`/api/outputs/${id}/preview`))
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
      await api.put(`/api/outputs/${output.id}`, {
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
