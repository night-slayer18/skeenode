import { useState } from 'react'
import { motion } from 'framer-motion'
import { User, Key, Palette, Shield, Plus, Trash2, Copy, Eye, EyeOff } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import Header from '@/components/layout/Header'
import { useAuth } from '@/lib/auth'

export default function SettingsPage() {
  const { user } = useAuth()
  const [apiKeys, setApiKeys] = useState([
    { id: '1', name: 'Production Key', prefix: 'sk_prod_', created: '2024-01-15' },
    { id: '2', name: 'Development Key', prefix: 'sk_dev_', created: '2024-02-20' },
  ])
  const [newKeyName, setNewKeyName] = useState('')
  const [showKey, setShowKey] = useState<string | null>(null)

  const handleCreateKey = () => {
    if (!newKeyName) return
    const newKey = {
      id: Date.now().toString(),
      name: newKeyName,
      prefix: 'sk_' + Math.random().toString(36).slice(2, 8) + '_',
      created: new Date().toISOString().split('T')[0],
    }
    setApiKeys([...apiKeys, newKey])
    setNewKeyName('')
  }

  const handleDeleteKey = (id: string) => {
    if (confirm('Are you sure you want to revoke this API key?')) {
      setApiKeys(apiKeys.filter(k => k.id !== id))
    }
  }

  const containerVariants = {
    hidden: { opacity: 0 },
    visible: { opacity: 1, transition: { staggerChildren: 0.1 } }
  }

  const itemVariants = {
    hidden: { opacity: 0, y: 20 },
    visible: { opacity: 1, y: 0 }
  }

  return (
    <div className="min-h-screen">
      <Header title="Settings" description="Manage your account and preferences" />

      <motion.div
        variants={containerVariants}
        initial="hidden"
        animate="visible"
        className="p-6 space-y-6 max-w-4xl"
      >
        {/* Profile Section */}
        <motion.div variants={itemVariants}>
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <User className="w-5 h-5 text-primary" />
                Profile
              </CardTitle>
              <CardDescription>Your account information</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="flex items-center gap-4">
                <div className="w-16 h-16 rounded-full bg-primary/20 flex items-center justify-center text-primary text-2xl font-bold">
                  {user?.username?.charAt(0).toUpperCase() || 'U'}
                </div>
                <div>
                  <h3 className="text-lg font-semibold">{user?.username || 'User'}</h3>
                  <p className="text-muted-foreground">{user?.email || 'user@example.com'}</p>
                  <Badge variant="success" className="mt-1">{user?.role || 'admin'}</Badge>
                </div>
              </div>
            </CardContent>
          </Card>
        </motion.div>

        {/* API Keys Section */}
        <motion.div variants={itemVariants}>
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <Key className="w-5 h-5 text-primary" />
                API Keys
              </CardTitle>
              <CardDescription>Manage your API keys for programmatic access</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              {/* Create new key */}
              <div className="flex gap-2">
                <Input
                  placeholder="Key name (e.g., CI/CD Pipeline)"
                  value={newKeyName}
                  onChange={(e) => setNewKeyName(e.target.value)}
                  className="flex-1"
                />
                <Button onClick={handleCreateKey} disabled={!newKeyName}>
                  <Plus className="w-4 h-4 mr-2" />
                  Create Key
                </Button>
              </div>

              {/* Keys list */}
              <div className="space-y-2">
                {apiKeys.map((key) => (
                  <motion.div
                    key={key.id}
                    initial={{ opacity: 0, x: -10 }}
                    animate={{ opacity: 1, x: 0 }}
                    className="flex items-center justify-between p-3 rounded-lg bg-secondary/30 border border-border"
                  >
                    <div className="flex items-center gap-3">
                      <Shield className="w-4 h-4 text-muted-foreground" />
                      <div>
                        <p className="font-medium text-sm">{key.name}</p>
                        <div className="flex items-center gap-2">
                          <code className="text-xs text-muted-foreground">
                            {showKey === key.id ? key.prefix + '••••••••••••' : key.prefix + '••••••••'}
                          </code>
                          <button onClick={() => setShowKey(showKey === key.id ? null : key.id)}>
                            {showKey === key.id ? (
                              <EyeOff className="w-3 h-3 text-muted-foreground" />
                            ) : (
                              <Eye className="w-3 h-3 text-muted-foreground" />
                            )}
                          </button>
                        </div>
                      </div>
                    </div>
                    <div className="flex items-center gap-2">
                      <span className="text-xs text-muted-foreground">
                        Created {key.created}
                      </span>
                      <Button variant="ghost" size="icon" className="h-8 w-8">
                        <Copy className="w-4 h-4" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-8 w-8 text-destructive"
                        onClick={() => handleDeleteKey(key.id)}
                      >
                        <Trash2 className="w-4 h-4" />
                      </Button>
                    </div>
                  </motion.div>
                ))}
              </div>
            </CardContent>
          </Card>
        </motion.div>

        {/* Preferences */}
        <motion.div variants={itemVariants}>
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <Palette className="w-5 h-5 text-primary" />
                Preferences
              </CardTitle>
              <CardDescription>Customize your experience</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="flex items-center justify-between">
                <div>
                  <p className="font-medium">Theme</p>
                  <p className="text-sm text-muted-foreground">Choose your preferred color scheme</p>
                </div>
                <div className="flex gap-2">
                  <button className="w-8 h-8 rounded-full bg-gradient-to-br from-primary to-accent border-2 border-primary" />
                  <button className="w-8 h-8 rounded-full bg-slate-800 border border-border" />
                  <button className="w-8 h-8 rounded-full bg-white border border-border" />
                </div>
              </div>
            </CardContent>
          </Card>
        </motion.div>
      </motion.div>
    </div>
  )
}
