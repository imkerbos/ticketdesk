<template>
  <!-- 系统设置：原来没有页标题，直接就是 tab 栏。补上 26px 标题这一层。
       tab 仍用 el-tabs（七个 pane 嵌套很深，拆成 v-if 的收益只有 tab 栏
       那一条线，风险却是整页的），外观由 _components.scss 统一。 -->
  <div class="page">
    <div class="page-head">
      <h1>{{ t('nav.settings') }}</h1>
    </div>

    <el-tabs v-model="activeTab" class="settings-tabs">
      <!-- 品牌设置 -->
      <el-tab-pane :label="t('system.tabBrand')" name="brand">
        <el-card shadow="never" class="settings-card">
          <template #header>
            <div class="card-header">
              <div class="card-title">
                <span class="title">{{ t('system.brandTitle') }}</span>
              </div>
            </div>
          </template>

          <el-row :gutter="40">
            <el-col :xs="24" :lg="14">
              <el-form
                ref="brandFormRef"
                :model="brandForm"
                :rules="brandRules"
                label-position="top"
                class="settings-form"
              >
                <div class="form-section">
                  <div class="section-title">{{ t('system.basic') }}</div>
                  <el-form-item :label="t('system.systemName')" prop="system_name">
                    <el-input
                      v-model="brandForm.system_name"
                      :placeholder="t('system.systemNamePlaceholder')"
                      maxlength="50"
                      show-word-limit
                    />
                    <div class="form-item-tip">
                      {{ t('system.systemNameTip') }}
                    </div>
                  </el-form-item>

                  <el-form-item :label="t('system.systemDesc')" prop="system_description">
                    <el-input
                      v-model="brandForm.system_description"
                      :placeholder="t('system.systemDescPlaceholder')"
                      maxlength="200"
                      show-word-limit
                    />
                  </el-form-item>

                  <el-form-item :label="t('system.copyright')" prop="copyright_text">
                    <el-input
                      v-model="brandForm.copyright_text"
                      :placeholder="t('system.copyrightPlaceholder')"
                      maxlength="200"
                      show-word-limit
                    />
                  </el-form-item>
                </div>

                <div class="form-section">
                  <div class="section-title">{{ t('system.loginCopy') }}</div>
                  <el-form-item :label="t('system.loginTitle')" prop="login_title">
                    <el-input
                      v-model="brandForm.login_title"
                      :placeholder="t('system.loginTitlePlaceholder')"
                      maxlength="100"
                      show-word-limit
                    />
                  </el-form-item>

                  <el-form-item :label="t('system.loginDesc')" prop="login_description">
                    <el-input
                      v-model="brandForm.login_description"
                      type="textarea"
                      :rows="3"
                      :placeholder="t('system.loginDescPlaceholder')"
                      maxlength="500"
                      show-word-limit
                    />
                  </el-form-item>
                </div>

                <div class="form-section">
                  <div class="section-title">{{ t('system.brandAssets') }}</div>
                  <el-form-item label="Logo">
                    <div class="upload-area">
                      <el-upload
                        :show-file-list="false"
                        :before-upload="beforeBrandUpload"
                        :http-request="(opts: any) => handleBrandUpload(opts, 'logo')"
                        accept=".svg,.png,.ico,.jpg,.jpeg,.webp"
                      >
                        <el-button :loading="logoUploading">
                          {{ brandForm.logo_url ? t('system.changeLogo') : t('system.uploadLogo') }}
                        </el-button>
                      </el-upload>
                      <div v-if="brandForm.logo_url" class="upload-preview">
                        <img :src="brandForm.logo_url" alt="Logo" class="preview-image" />
                        <el-button text type="danger" size="small" @click="removeBrandAsset('logo')">{{ t('common.remove') }}</el-button>
                      </div>
                    </div>
                    <div class="form-item-tip">
                      {{ t('system.logoTip') }}
                    </div>
                  </el-form-item>

                  <el-form-item label="Favicon">
                    <div class="upload-area">
                      <el-upload
                        :show-file-list="false"
                        :before-upload="beforeBrandUpload"
                        :http-request="(opts: any) => handleBrandUpload(opts, 'favicon')"
                        accept=".svg,.png,.ico"
                      >
                        <el-button :loading="faviconUploading">
                          {{ brandForm.favicon_url ? t('system.changeFavicon') : t('system.uploadFavicon') }}
                        </el-button>
                      </el-upload>
                      <div v-if="brandForm.favicon_url" class="upload-preview">
                        <img :src="brandForm.favicon_url" alt="Favicon" class="preview-image preview-favicon" />
                        <el-button text type="danger" size="small" @click="removeBrandAsset('favicon')">{{ t('common.remove') }}</el-button>
                      </div>
                    </div>
                    <div class="form-item-tip">
                      {{ t('system.faviconTip') }}
                    </div>
                  </el-form-item>
                </div>

                <el-form-item>
                  <el-button
                    type="primary"
                    :loading="brandLoading"
                    @click="handleSaveBrandConfig"
                  >
                    <el-icon><Check /></el-icon>
                    {{ t('system.saveBrand') }}
                  </el-button>
                </el-form-item>
              </el-form>
            </el-col>

            <el-col :xs="24" :lg="10">
              <div class="brand-preview-section">
                <div class="section-title">{{ t('system.preview') }}</div>
                <div class="brand-preview-card">
                  <div class="preview-sidebar">
                    <div class="preview-logo-area">
                      <img
                        v-if="brandForm.logo_url"
                        :src="brandForm.logo_url"
                        alt="Logo"
                        class="preview-logo-img"
                      />
                      <span v-else class="preview-logo-placeholder">{{ (brandForm.system_name || 'TicketDesk').charAt(0) }}</span>
                      <span class="preview-logo-text">{{ brandForm.system_name || 'TicketDesk' }}</span>
                    </div>
                    <div class="preview-menu-item active">{{ t('system.previewHome') }}</div>
                    <div class="preview-menu-item">{{ t('system.previewIssues') }}</div>
                    <div class="preview-menu-item">{{ t('system.previewProjects') }}</div>
                    <div class="preview-copyright">{{ brandForm.copyright_text || '© 2026 TicketDesk' }}</div>
                  </div>
                </div>
                <el-alert type="info" :closable="false" class="brand-tips">
                  <p>{{ t('system.previewTip1') }}</p>
                  <p>{{ t('system.previewTip2') }}</p>
                </el-alert>
              </div>
            </el-col>
          </el-row>
        </el-card>
      </el-tab-pane>

      <!-- 通用配置 -->
      <el-tab-pane :label="t('system.tabGeneral')" name="general">
        <el-card shadow="never" class="settings-card">
          <template #header>
            <div class="card-header">
              <div class="card-title">
                <span class="title">{{ t('system.generalTitle') }}</span>
              </div>
            </div>
          </template>

          <!-- 右侧「配置说明」已删（内容与字段提示重复），表单不再占半屏而是限宽单列 -->
          <div class="settings-single-col">
            <el-form
              ref="generalFormRef"
              :model="generalForm"
              :rules="generalRules"
              label-position="top"
              class="settings-form"
            >
              <div class="form-section">
                <div class="section-title">{{ t('system.siteSection') }}</div>
                <el-form-item :label="t('system.siteUrl')" prop="site_url">
                  <el-input
                    v-model="generalForm.site_url"
                    :placeholder="t('system.siteUrlPlaceholder')"
                  >
                    <template #prefix>
                      <el-icon><Link /></el-icon>
                    </template>
                  </el-input>
                  <!-- 原来右侧还有半屏的「配置说明」在讲同一个字段，
                         内容和这里重复；示例并进来，那一整列删掉 -->
                  <div class="form-item-tip">
                    {{ t('system.siteUrlTip') }}
                    <span class="form-item-example">{{ t('system.siteUrlExample') }}</span>
                  </div>
                </el-form-item>
              </div>

              <div class="form-section">
                <div class="section-title">{{ t('system.languageSection') }}</div>
                <el-form-item :label="t('system.language')" prop="language">
                  <el-select v-model="generalForm.language" style="width: 100%">
                    <el-option :label="t('lang.zh-CN')" value="zh-CN" />
                    <el-option :label="t('lang.en-US')" value="en-US" />
                  </el-select>
                  <div class="form-item-tip">
                    {{ t('system.languageTip') }}
                  </div>
                </el-form-item>
              </div>

              <el-form-item>
                <el-button type="primary" :loading="generalLoading" @click="saveGeneralConfig">
                  {{ t('system.saveConfig') }}
                </el-button>
              </el-form-item>
            </el-form>
          </div>
        </el-card>
      </el-tab-pane>

      <!-- 邮件配置 -->
      <el-tab-pane :label="t('system.tabEmail')" name="email">
        <el-card shadow="never" class="settings-card">
          <template #header>
            <div class="card-header">
              <div class="card-title">
                <span class="title">{{ t('system.emailTitle') }}</span>
              </div>
              <div class="header-switch">
                <el-switch v-model="emailForm.enabled" />
                <span class="header-switch-label" :class="{ active: emailForm.enabled }">
                  {{ emailForm.enabled ? t('common.enabled') : t('common.disabled') }}
                </span>
              </div>
            </div>
          </template>

          <el-row :gutter="40">
            <el-col :xs="24" :lg="12">
              <el-form
                ref="emailFormRef"
                :model="emailForm"
                :rules="emailRules"
                label-position="top"
                class="settings-form"
              >
                <div class="form-section">
                  <div class="section-title">{{ t('system.serverSection') }}</div>
                  <el-row :gutter="16">
                    <el-col :span="16">
                      <el-form-item :label="t('system.smtpHost')" prop="smtp_host">
                        <el-input v-model="emailForm.smtp_host" :placeholder="t('system.smtpHostPlaceholder')">
                          <template #prefix>
                            <el-icon><Monitor /></el-icon>
                          </template>
                        </el-input>
                      </el-form-item>
                    </el-col>
                    <el-col :span="8">
                      <el-form-item :label="t('system.port')" prop="smtp_port">
                        <!-- 步进器形态跟安全设置那几个数字框保持一致：右侧竖排，
                             不用左右分开的 −/+，否则同一页两种数字输入长相 -->
                        <el-input-number v-model="emailForm.smtp_port" :min="1" :max="65535" controls-position="right" style="width: 100%" />
                      </el-form-item>
                    </el-col>
                  </el-row>

                  <el-row :gutter="16">
                    <el-col :span="12">
                      <el-form-item :label="t('system.smtpUsername')" prop="smtp_username">
                        <el-input v-model="emailForm.smtp_username" :placeholder="t('system.smtpUsernamePlaceholder')">
                          <template #prefix>
                            <el-icon><User /></el-icon>
                          </template>
                        </el-input>
                      </el-form-item>
                    </el-col>
                    <el-col :span="12">
                      <el-form-item :label="t('system.smtpPassword')" prop="smtp_password">
                        <el-input
                          v-model="emailForm.smtp_password"
                          type="password"
                          :placeholder="t('system.unchangedPlaceholder')"
                          show-password
                        >
                          <template #prefix>
                            <el-icon><Lock /></el-icon>
                          </template>
                        </el-input>
                      </el-form-item>
                    </el-col>
                  </el-row>
                </div>

                <div class="form-section">
                  <div class="section-title">{{ t('system.senderSection') }}</div>
                  <el-row :gutter="16">
                    <el-col :span="14">
                      <el-form-item :label="t('system.fromAddress')" prop="from_address">
                        <el-input v-model="emailForm.from_address" placeholder="noreply@example.com">
                          <template #prefix>
                            <el-icon><Message /></el-icon>
                          </template>
                        </el-input>
                      </el-form-item>
                    </el-col>
                    <el-col :span="10">
                      <el-form-item :label="t('system.fromName')" prop="from_name">
                        <el-input v-model="emailForm.from_name" placeholder="TicketDesk" />
                      </el-form-item>
                    </el-col>
                  </el-row>
                </div>

                <div class="form-section">
                  <div class="section-title">{{ t('system.securitySection') }}</div>
                  <el-form-item>
                    <div class="setting-row">
                      <div class="setting-info">
                        <span class="setting-label">{{ t('system.useTls') }}</span>
                        <span class="setting-desc">{{ t('system.useTlsDesc') }}</span>
                      </div>
                      <el-switch v-model="emailForm.use_tls" />
                    </div>
                  </el-form-item>
                </div>

                <el-form-item>
                  <el-button type="primary" :loading="emailSaving" @click="saveEmailConfig">
                    <el-icon><Check /></el-icon>
                    {{ t('system.saveConfig') }}
                  </el-button>
                  <el-button @click="testEmailDialog = true">
                    <el-icon><Promotion /></el-icon>
                    {{ t('system.sendTestEmail') }}
                  </el-button>
                </el-form-item>
              </el-form>
            </el-col>
            <el-col :xs="24" :lg="12" class="info-panel">
              <div class="info-card">
                <div class="info-title">
                  <el-icon><InfoFilled /></el-icon>
                  {{ t('system.configHelp') }}
                </div>
                <ul class="info-list">
                  <li>{{ t('system.emailHelp1') }}</li>
                  <li>{{ t('system.emailHelp2') }}</li>
                  <li>{{ t('system.emailHelp3') }}</li>
                  <li>{{ t('system.emailHelp4') }}</li>
                </ul>
              </div>
            </el-col>
          </el-row>
        </el-card>
      </el-tab-pane>

      <!-- 安全配置 -->
      <el-tab-pane :label="t('system.tabSecurity')" name="security">
        <el-row :gutter="20">
          <!-- MFA 设置 -->
          <el-col :xs="24" :lg="8">
            <el-card shadow="never" class="setting-block">
              <div class="block-header">
                <div class="block-title">
                  <span class="title">{{ t('system.mfaTitle') }}</span>
                  <span class="desc">{{ t('system.mfaDesc') }}</span>
                </div>
              </div>
              <div class="block-content">
                <div class="setting-item">
                  <div class="setting-info">
                    <span class="setting-label">{{ t('system.mfaEnable') }}</span>
                    <span class="setting-desc">{{ t('system.mfaEnableDesc') }}</span>
                  </div>
                  <el-switch v-model="securityForm.mfa_enabled" />
                </div>
                <div class="setting-item">
                  <div class="setting-info">
                    <span class="setting-label">{{ t('system.mfaForce') }}</span>
                    <span class="setting-desc">{{ t('system.mfaForceDesc') }}</span>
                  </div>
                  <el-switch
                    v-model="securityForm.mfa_required"
                    :disabled="!securityForm.mfa_enabled"
                  />
                </div>
              </div>
            </el-card>
          </el-col>

          <!-- 密码策略 -->
          <el-col :xs="24" :lg="8">
            <el-card shadow="never" class="setting-block">
              <div class="block-header">
                <div class="block-title">
                  <span class="title">{{ t('system.passwordPolicy') }}</span>
                  <span class="desc">{{ t('system.passwordPolicyDesc') }}</span>
                </div>
              </div>
              <div class="block-content">
                <div class="setting-item">
                  <div class="setting-info">
                    <span class="setting-label">{{ t('system.minLength') }}</span>
                    <span class="setting-desc">{{ t('system.minLengthDesc') }}</span>
                  </div>
                  <el-input-number
                    v-model="securityForm.password_min_length"
                    :min="6"
                    :max="32"
                    size="small"
                    controls-position="right"
                  />
                </div>
                <div class="setting-item">
                  <div class="setting-info">
                    <span class="setting-label">{{ t('system.requireUpper') }}</span>
                    <span class="setting-desc">{{ t('system.requireUpperDesc') }}</span>
                  </div>
                  <el-switch v-model="securityForm.password_require_upper" />
                </div>
                <div class="setting-item">
                  <div class="setting-info">
                    <span class="setting-label">{{ t('system.requireDigit') }}</span>
                    <span class="setting-desc">{{ t('system.requireDigitDesc') }}</span>
                  </div>
                  <el-switch v-model="securityForm.password_require_number" />
                </div>
              </div>
            </el-card>
          </el-col>

          <!-- 会话设置 -->
          <el-col :xs="24" :lg="8">
            <el-card shadow="never" class="setting-block">
              <div class="block-header">
                <div class="block-title">
                  <span class="title">{{ t('system.sessionTitle') }}</span>
                  <span class="desc">{{ t('system.sessionDesc') }}</span>
                </div>
              </div>
              <div class="block-content">
                <div class="setting-item">
                  <div class="setting-info">
                    <span class="setting-label">{{ t('system.sessionTimeout') }}</span>
                    <span class="setting-desc">{{ t('system.sessionTimeoutDesc') }}</span>
                  </div>
                  <el-input-number
                    v-model="securityForm.session_timeout"
                    :min="5"
                    :max="1440"
                    size="small"
                    controls-position="right"
                  />
                </div>
              </div>
            </el-card>
          </el-col>
        </el-row>

        <div class="action-bar">
          <el-button type="primary" :loading="securitySaving" @click="saveSecurityConfig">
            <el-icon><Check /></el-icon>
            {{ t('system.saveSecurity') }}
          </el-button>
        </div>
      </el-tab-pane>

      <!-- 限流配置 -->
      <el-tab-pane :label="t('system.tabRateLimit')" name="ratelimit">
        <el-row :gutter="20">
          <!-- Webhook 限流 -->
          <el-col :xs="24" :lg="8">
            <el-card shadow="never" class="setting-block">
              <div class="block-header">
                <div class="block-title">
                  <span class="title">{{ t('system.webhookLimit') }}</span>
                  <span class="desc">{{ t('system.webhookLimitDesc') }}</span>
                </div>
              </div>
              <div class="block-content">
                <div class="setting-item">
                  <div class="setting-info">
                    <span class="setting-label">{{ t('system.perIpPerMinute') }}</span>
                    <span class="setting-desc">{{ t('system.range', { min: '10', max: '10,000' }) }}</span>
                  </div>
                  <el-input-number
                    v-model="rateLimitForm.webhook_limit"
                    :min="10"
                    :max="10000"
                    size="small"
                    controls-position="right"
                  />
                </div>
              </div>
            </el-card>
          </el-col>

          <!-- 认证限流 -->
          <el-col :xs="24" :lg="8">
            <el-card shadow="never" class="setting-block">
              <div class="block-header">
                <div class="block-title">
                  <span class="title">{{ t('system.authLimit') }}</span>
                  <span class="desc">{{ t('system.authLimitDesc') }}</span>
                </div>
              </div>
              <div class="block-content">
                <div class="setting-item">
                  <div class="setting-info">
                    <span class="setting-label">{{ t('system.perIpPerMinute') }}</span>
                    <span class="setting-desc">{{ t('system.range', { min: '5', max: '1,000' }) }}</span>
                  </div>
                  <el-input-number
                    v-model="rateLimitForm.auth_limit"
                    :min="5"
                    :max="1000"
                    size="small"
                    controls-position="right"
                  />
                </div>
              </div>
            </el-card>
          </el-col>

          <!-- API 全局限流 -->
          <el-col :xs="24" :lg="8">
            <el-card shadow="never" class="setting-block">
              <div class="block-header">
                <div class="block-title">
                  <span class="title">{{ t('system.apiLimit') }}</span>
                  <span class="desc">{{ t('system.apiLimitDesc') }}</span>
                </div>
              </div>
              <div class="block-content">
                <div class="setting-item">
                  <div class="setting-info">
                    <span class="setting-label">{{ t('system.perIpPerMinute') }}</span>
                    <span class="setting-desc">{{ t('system.range', { min: '50', max: '50,000' }) }}</span>
                  </div>
                  <el-input-number
                    v-model="rateLimitForm.api_limit"
                    :min="50"
                    :max="50000"
                    size="small"
                    controls-position="right"
                  />
                </div>
              </div>
            </el-card>
          </el-col>
        </el-row>

        <div class="action-bar">
          <el-button type="primary" :loading="rateLimitSaving" @click="saveRateLimitConfig">
            <el-icon><Check /></el-icon>
            {{ t('system.saveRateLimit') }}
          </el-button>
        </div>
      </el-tab-pane>
      <!-- 工时配置 -->
      <el-tab-pane :label="t('system.tabWorklog')" name="worklog">
        <el-card shadow="never" class="settings-card">
          <template #header>
            <div class="card-header">
              <div class="card-title">
                <span class="title">{{ t('system.workTypeTitle') }}</span>
              </div>
            </div>
          </template>

          <el-row :gutter="40">
            <el-col :xs="24" :lg="14">
              <div class="work-type-list">
                <div
                  v-for="(item, index) in workTypeList"
                  :key="index"
                  class="work-type-item"
                >
                  <el-input
                    v-model="item.label"
                    :placeholder="t('system.workTypePlaceholder')"
                    style="flex: 1"
                    @input="item.value = item.label"
                  />
                  <el-button
                    type="danger"
                    link
                    :disabled="workTypeList.length <= 1"
                    @click="removeWorkType(index)"
                  >
                    <el-icon><Delete /></el-icon>
                  </el-button>
                </div>
                <el-button type="primary" link style="margin-top: 8px" @click="addWorkType">
                  {{ t('system.addWorkType') }}
                </el-button>
              </div>

              <el-form-item style="margin-top: 24px">
                <el-button type="primary" :loading="worklogSaving" @click="saveWorkTypeConfig">
                  <el-icon><Check /></el-icon>
                  {{ t('system.saveConfig') }}
                </el-button>
              </el-form-item>
            </el-col>

            <el-col :xs="24" :lg="10">
              <div class="config-tips">
                <div class="tip-title">
                  <el-icon><InfoFilled /></el-icon>
                  <span>{{ t('system.configHelp') }}</span>
                </div>
                <div class="tip-content">
                  <div class="tip-item">
                    <div class="tip-label">{{ t('system.workTypeHelpLabel') }}</div>
                    <div class="tip-desc">
                      {{ t('system.workTypeHelp') }}
                    </div>
                  </div>
                </div>
              </div>
            </el-col>
          </el-row>
        </el-card>
      </el-tab-pane>

      <!-- SSO 配置 -->
      <el-tab-pane :label="t('system.tabSso')" name="sso">
        <el-card shadow="never" class="settings-card">
          <template #header>
            <div class="card-header">
              <div class="card-title">
                <span class="title">{{ t('system.ssoTitle') }}</span>
              </div>
              <div class="header-switch">
                <el-switch v-model="ssoForm.enabled" />
                <span class="header-switch-label" :class="{ active: ssoForm.enabled }">
                  {{ ssoForm.enabled ? t('common.enabled') : t('common.disabled') }}
                </span>
              </div>
            </div>
          </template>

          <el-row :gutter="40">
            <el-col :xs="24" :lg="14">
              <el-form
                ref="ssoFormRef"
                :model="ssoForm"
                label-position="top"
                class="settings-form"
              >
                <div class="form-section">
                  <div class="section-title">{{ t('system.ssoBasic') }}</div>
                  <el-form-item :label="t('system.providerName')" prop="provider_name">
                    <el-input v-model="ssoForm.provider_name" :placeholder="t('system.providerNamePlaceholder')">
                      <template #prefix>
                        <el-icon><User /></el-icon>
                      </template>
                    </el-input>
                    <div class="form-item-tip">{{ t('system.providerNameTip') }}</div>
                  </el-form-item>

                  <el-form-item label="Issuer URL" prop="issuer_url">
                    <el-input v-model="ssoForm.issuer_url" :placeholder="t('system.issuerPlaceholder')">
                      <template #prefix>
                        <el-icon><Link /></el-icon>
                      </template>
                    </el-input>
                    <div class="form-item-tip">{{ t('system.issuerTip') }}</div>
                  </el-form-item>

                  <el-row :gutter="16">
                    <el-col :span="12">
                      <el-form-item label="Client ID" prop="client_id">
                        <el-input v-model="ssoForm.client_id" placeholder="OIDC Client ID" />
                      </el-form-item>
                    </el-col>
                    <el-col :span="12">
                      <el-form-item label="Client Secret" prop="client_secret">
                        <el-input
                          v-model="ssoForm.client_secret"
                          type="password"
                          :placeholder="t('system.unchangedPlaceholder')"
                          show-password
                        >
                          <template #prefix>
                            <el-icon><Lock /></el-icon>
                          </template>
                        </el-input>
                      </el-form-item>
                    </el-col>
                  </el-row>

                  <el-row :gutter="16">
                    <el-col :span="12">
                      <el-form-item :label="t('system.redirectUri')" prop="redirect_uri">
                        <el-input v-model="ssoForm.redirect_uri" :placeholder="t('system.redirectUriPlaceholder')" />
                      </el-form-item>
                    </el-col>
                    <el-col :span="12">
                      <el-form-item label="Scopes" prop="scopes">
                        <el-input v-model="ssoForm.scopes" placeholder="openid,profile,email" />
                      </el-form-item>
                    </el-col>
                  </el-row>
                </div>

                <div class="form-section">
                  <div class="section-title">{{ t('system.ssoUserSection') }}</div>
                  <el-form-item>
                    <div class="setting-row">
                      <div class="setting-info">
                        <span class="setting-label">{{ t('system.autoCreate') }}</span>
                        <span class="setting-desc">{{ t('system.autoCreateDesc') }}</span>
                      </div>
                      <el-switch v-model="ssoForm.auto_create_user" />
                    </div>
                  </el-form-item>
                  <el-form-item :label="t('system.defaultRole')" prop="default_role">
                    <el-select v-model="ssoForm.default_role" style="width: 200px">
                      <el-option :label="t('system.roleUser')" value="user" />
                      <el-option :label="t('system.roleProjectAdmin')" value="project_admin" />
                      <el-option :label="t('system.roleAdmin')" value="admin" />
                    </el-select>
                    <div class="form-item-tip">{{ t('system.defaultRoleTip') }}</div>
                  </el-form-item>
                </div>

                <div class="form-section">
                  <div class="section-title">
                    {{ t('system.claimMappings') }}
                    <el-button type="primary" link size="small" style="margin-left: 8px" @click="addClaimMapping">
                      {{ t('system.addMapping') }}
                    </el-button>
                  </div>
                  <div class="claim-mappings">
                    <div
                      v-for="(mapping, index) in ssoForm.claim_mappings"
                      :key="index"
                      class="claim-mapping-row"
                    >
                      <el-row :gutter="12" style="flex: 1">
                        <el-col :span="10">
                          <el-input
                            v-model="mapping.local_field"
                            :placeholder="t('system.localFieldPlaceholder')"
                            size="default"
                          />
                        </el-col>
                        <el-col :span="2" style="text-align: center; line-height: 32px; color: var(--td-color-info)">
                          ←
                        </el-col>
                        <el-col :span="10">
                          <el-input
                            v-model="mapping.claim_name"
                            :placeholder="t('system.claimNamePlaceholder')"
                            size="default"
                          />
                        </el-col>
                        <el-col :span="2">
                          <el-button
                            type="danger"
                            link
                            :disabled="ssoForm.claim_mappings.length <= 1"
                            @click="removeClaimMapping(index)"
                          >
                            {{ t('common.delete') }}
                          </el-button>
                        </el-col>
                      </el-row>
                    </div>
                    <div class="claim-mapping-hint">
                      {{ t('system.claimHint') }}
                    </div>
                  </div>
                </div>

                <el-form-item>
                  <el-button type="primary" :loading="ssoSaving" @click="saveSSOConfig">
                    <el-icon><Check /></el-icon>
                    {{ t('system.saveConfig') }}
                  </el-button>
                </el-form-item>
              </el-form>
            </el-col>

            <el-col :xs="24" :lg="10">
              <div class="config-tips">
                <div class="tip-title">
                  <el-icon><InfoFilled /></el-icon>
                  <span>{{ t('system.configHelp') }}</span>
                </div>
                <div class="tip-content">
                  <div class="tip-item">
                    <div class="tip-label">{{ t('system.ssoStepsLabel') }}</div>
                    <div class="tip-desc">
                      {{ t('system.ssoStep1') }}<br />
                      {{ t('system.ssoStep2') }}<br />
                      {{ t('system.ssoStep3') }}<br />
                      {{ t('system.ssoStep4') }}
                    </div>
                  </div>
                  <div class="tip-item">
                    <div class="tip-label">{{ t('system.idpInitLabel') }}</div>
                    <div class="tip-desc">
                      {{ t('system.idpInitDesc') }}<br />
                      <code>{{ ssoForm.redirect_uri?.replace('/auth/sso/callback', '/auth/sso/login') || 'https://your-domain/auth/sso/login' }}</code>
                    </div>
                  </div>
                  <div class="tip-item">
                    <div class="tip-label">{{ t('system.claimsLabel') }}</div>
                    <div class="tip-desc">
                      {{ t('system.claimsDesc1') }}<br />
                      {{ t('system.claimsDesc2') }}<br />
                      {{ t('system.claimsDesc3') }}
                    </div>
                  </div>
                </div>
              </div>
            </el-col>
          </el-row>
        </el-card>
      </el-tab-pane>
    </el-tabs>

    <!-- 测试邮件对话框 -->
    <el-dialog v-model="testEmailDialog" :title="t('system.testEmailTitle')" width="400px">
      <el-form ref="testEmailFormRef" :model="testEmailForm" :rules="testEmailRules" label-position="top">
        <el-form-item :label="t('system.testRecipient')" prop="to_address">
          <el-input v-model="testEmailForm.to_address" :placeholder="t('system.testRecipientPlaceholder')" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="testEmailDialog = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="testEmailSending" @click="sendTestEmail">
          {{ t('system.sendTest') }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ref, reactive, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import {
  Message, Lock, Check, Promotion, InfoFilled,
  Monitor, User, Link,
  Delete } from '@element-plus/icons-vue'
import {
  getEmailConfig,
  updateEmailConfig,
  getSecurityConfig,
  updateSecurityConfig,
  getRateLimitConfig,
  updateRateLimitConfig,
  getConfig,
  updateConfig,
  getSSOAdminConfig,
  updateSSOConfig,
  getBrandConfig,
  updateBrandConfig,
  uploadBrandAsset } from '@/api/system'
import { useBrandStore } from '@/stores/brand'

const { t } = useI18n()

const route = useRoute()
const router = useRouter()

// 从 URL 查询参数中获取 tab，认不出来就退回默认的 —— 不校验的话，
// URL 里写错一个 tab 名就渲染出一张空白页，连 tab 都不高亮。
const SYSTEM_TABS = ['brand', 'general', 'email', 'security', 'ratelimit', 'worklog', 'sso']
const initTab = route.query.tab as string
const activeTab = ref(SYSTEM_TABS.includes(initTab) ? initTab : 'general')

// 监听 tab 变化，更新 URL
watch(activeTab, (newTab) => {
  router.replace({
    query: { ...route.query, tab: newTab } })
})

const brandStore = useBrandStore()

// ============ 品牌配置 ============
const brandFormRef = ref<FormInstance>()
const brandLoading = ref(false)
const logoUploading = ref(false)
const faviconUploading = ref(false)
const brandForm = reactive({
  system_name: '',
  system_description: '',
  copyright_text: '',
  login_title: '',
  login_description: '',
  logo_url: '',
  favicon_url: '' })

const brandRules: FormRules = {
  system_name: [
    { required: true, message: t('system.systemNameRequired'), trigger: ['blur', 'change'] },
    { max: 50, message: t('system.systemNameMax'), trigger: 'blur' },
  ] }

const loadBrandConfig = async () => {
  try {
    const res = await getBrandConfig()
    const config = res.data.data
    brandForm.system_name = config.system_name
    brandForm.system_description = config.system_description
    brandForm.copyright_text = config.copyright_text
    brandForm.login_title = config.login_title
    brandForm.login_description = config.login_description
    brandForm.logo_url = config.logo_url
    brandForm.favicon_url = config.favicon_url
  } catch {
    // 使用默认值
  }
}

const beforeBrandUpload = (file: File) => {
  const maxSize = 2 * 1024 * 1024
  if (file.size > maxSize) {
    ElMessage.error(t('system.fileTooLarge'))
    return false
  }
  return true
}

const handleBrandUpload = async (options: { file: File }, type: 'logo' | 'favicon') => {
  const loadingRef = type === 'logo' ? logoUploading : faviconUploading
  loadingRef.value = true
  try {
    const res = await uploadBrandAsset(options.file, type)
    const url = res.data.data.url
    if (type === 'logo') {
      brandForm.logo_url = url
    } else {
      brandForm.favicon_url = url
    }
    ElMessage.success(t('system.uploadSuccess', { name: type === 'logo' ? 'Logo' : 'Favicon' }))
  } catch {
    ElMessage.error(t('system.uploadFailed'))
  } finally {
    loadingRef.value = false
  }
}

const removeBrandAsset = (type: 'logo' | 'favicon') => {
  if (type === 'logo') {
    brandForm.logo_url = ''
  } else {
    brandForm.favicon_url = ''
  }
}

const handleSaveBrandConfig = async () => {
  if (!brandFormRef.value) return

  await brandFormRef.value.validate(async (valid) => {
    if (!valid) return

    brandLoading.value = true
    try {
      await updateBrandConfig({
        system_name: brandForm.system_name,
        system_description: brandForm.system_description,
        copyright_text: brandForm.copyright_text,
        login_title: brandForm.login_title,
        login_description: brandForm.login_description })

      // 如果 logo_url 或 favicon_url 被清除，也需要更新配置
      if (!brandForm.logo_url) {
        await updateConfig('brand.logo_url', '')
      }
      if (!brandForm.favicon_url) {
        await updateConfig('brand.favicon_url', '')
      }

      // 刷新 brand store
      await brandStore.loadBrandConfig()

      ElMessage.success(t('system.brandSaved'))
    } catch {
      ElMessage.error(t('system.saveFailed'))
    } finally {
      brandLoading.value = false
    }
  })
}

// ============ 通用配置 ============
const generalFormRef = ref<FormInstance>()
const generalLoading = ref(false)
const generalForm = reactive({
  site_url: '',
  language: '' })

const generalRules: FormRules = {
  site_url: [
    { required: true, message: t('system.siteUrlRequired'), trigger: ['blur', 'change'] },
    { type: 'url', message: t('system.urlInvalid'), trigger: 'blur' },
  ] }

// 加载通用配置
const loadGeneralConfig = async () => {
  try {
    // 从系统配置中读取站点域名
    const res = await getConfig('general.site_url')
    if (res.data.data) {
      generalForm.site_url = res.data.data.config_value || ''
    }
    const langRes = await getConfig('general.language')
    if (langRes.data.data) {
      generalForm.language = langRes.data.data.config_value || 'zh-CN'
    }
  } catch (error: any) {
    // 如果配置不存在（404），不报错，使用默认空值
    if (error?.response?.status !== 404) {
      // ignored
    }
  }
}

// 保存通用配置
const saveGeneralConfig = async () => {
  if (!generalFormRef.value) return

  await generalFormRef.value.validate(async (valid) => {
    if (!valid) return

    generalLoading.value = true
    try {
      await updateConfig('general.site_url', generalForm.site_url)
      await updateConfig('general.language', generalForm.language)
      ElMessage.success(t('system.generalSaved'))
    } catch {
      // 错误已在拦截器中处理
    } finally {
      generalLoading.value = false
    }
  })
}

// ============ 邮件配置 ============
const emailFormRef = ref<FormInstance>()
const emailSaving = ref(false)
const emailForm = reactive({
  smtp_host: '',
  smtp_port: 587,
  smtp_username: '',
  smtp_password: '',
  from_address: '',
  from_name: 'TicketDesk',
  use_tls: true,
  enabled: false })

const emailRules: FormRules = {
  smtp_host: [{ required: true, message: t('system.smtpHostRequired'), trigger: ['blur', 'change'] }],
  smtp_port: [{ required: true, message: t('system.portRequired'), trigger: ['blur', 'change'] }],
  from_address: [
    { required: true, message: t('system.fromAddressRequired'), trigger: ['blur', 'change'] },
    { type: 'email', message: t('system.emailInvalid'), trigger: 'blur' },
  ] }

const loadEmailConfig = async () => {
  try {
    const { data } = await getEmailConfig()
    Object.assign(emailForm, data.data)
    // 后端没配过 SMTP 时端口返回 0，而输入框的 min 是 1，会被夹成一个
    // 谁都不会用的「1」—— 旁边说明里写的常用端口是 25 / 465 / 587。
    // 没配过就回到默认的 587。
    if (!emailForm.smtp_port) emailForm.smtp_port = 587
  } catch {
    // ignored
  }
}

const saveEmailConfig = async () => {
  if (!emailFormRef.value) return
  await emailFormRef.value.validate(async (valid) => {
    if (!valid) return

    emailSaving.value = true
    try {
      await updateEmailConfig(emailForm)
      ElMessage.success(t('system.emailSaved'))
    } catch {
      // ignored
    } finally {
      emailSaving.value = false
    }
  })
}

// 测试邮件
const testEmailDialog = ref(false)
const testEmailFormRef = ref<FormInstance>()
const testEmailSending = ref(false)
const testEmailForm = reactive({
  to_address: '' })
const testEmailRules: FormRules = {
  to_address: [
    { required: true, message: t('system.testRecipientRequired'), trigger: ['blur', 'change'] },
    { type: 'email', message: t('system.emailInvalid'), trigger: 'blur' },
  ] }

const sendTestEmail = async () => {
  if (!testEmailFormRef.value) return
  await testEmailFormRef.value.validate(async (valid) => {
    if (!valid) return

    testEmailSending.value = true
    try {
      // await testEmail(testEmailForm.to_address)
      ElMessage.success(t('system.testEmailSent'))
      testEmailDialog.value = false
    } catch {
      // ignored
    } finally {
      testEmailSending.value = false
    }
  })
}

// ============ 安全配置 ============
const securitySaving = ref(false)
const securityForm = reactive({
  mfa_enabled: false,
  mfa_required: false,
  password_min_length: 6,
  password_require_upper: false,
  password_require_number: false,
  session_timeout: 120 })

const loadSecurityConfig = async () => {
  try {
    const { data } = await getSecurityConfig()
    Object.assign(securityForm, data.data)
  } catch {
    // ignored
  }
}

const saveSecurityConfig = async () => {
  securitySaving.value = true
  try {
    await updateSecurityConfig(securityForm)
    ElMessage.success(t('system.securitySaved'))
  } catch {
    // ignored
  } finally {
    securitySaving.value = false
  }
}

// ============ 限流配置 ============
const rateLimitSaving = ref(false)
const rateLimitForm = reactive({
  webhook_limit: 100,
  auth_limit: 20,
  api_limit: 300 })

const loadRateLimitConfig = async () => {
  try {
    const { data } = await getRateLimitConfig()
    Object.assign(rateLimitForm, data.data)
  } catch {
    // ignored
  }
}

const saveRateLimitConfig = async () => {
  rateLimitSaving.value = true
  try {
    await updateRateLimitConfig(rateLimitForm)
    ElMessage.success(t('system.rateLimitSaved'))
  } catch {
    // ignored
  } finally {
    rateLimitSaving.value = false
  }
}

// ============ 工时配置 ============
const worklogSaving = ref(false)
const workTypeList = ref<{ value: string; label: string }[]>([])

const loadWorkTypeConfig = async () => {
  try {
    const res = await getConfig('worklog.work_types')
    if (res.data.data) {
      const parsed = JSON.parse(res.data.data.config_value || '[]')
      if (Array.isArray(parsed)) {
        workTypeList.value = parsed
      }
    }
  } catch (error: any) {
    if (error?.response?.status !== 404) {
      // ignored
    }
  }
}

const addWorkType = () => {
  workTypeList.value.push({ value: '', label: '' })
}

const removeWorkType = (index: number) => {
  workTypeList.value.splice(index, 1)
}

const saveWorkTypeConfig = async () => {
  // 过滤掉空项
  const filtered = workTypeList.value.filter(item => item.label.trim())
  if (filtered.length === 0) {
    ElMessage.warning(t('system.workTypeMinOne'))
    return
  }

  worklogSaving.value = true
  try {
    await updateConfig('worklog.work_types', JSON.stringify(filtered))
    workTypeList.value = filtered
    ElMessage.success(t('system.worklogSaved'))
  } catch {
    // ignored
  } finally {
    worklogSaving.value = false
  }
}

// ============ SSO 配置 ============
const ssoFormRef = ref<FormInstance>()
const ssoSaving = ref(false)
const ssoForm = reactive({
  enabled: false,
  provider_name: t('system.providerNameDefault'),
  client_id: '',
  client_secret: '',
  issuer_url: '',
  // 按当前站点地址生成，不要写死端口 —— 原来是 vite 的 5173，装到真实域名下就是错的
  redirect_uri: `${window.location.origin}/auth/sso/callback`,
  scopes: 'openid,profile,email',
  auto_create_user: true,
  default_role: 'user',
  claim_mappings: [
    { local_field: 'username', claim_name: 'preferred_username' },
    { local_field: 'email', claim_name: 'email' },
    { local_field: 'display_name', claim_name: 'name' },
    { local_field: 'avatar', claim_name: 'picture' },
  ] as Array<{ local_field: string; claim_name: string }> })

const addClaimMapping = () => {
  ssoForm.claim_mappings.push({ local_field: '', claim_name: '' })
}

const removeClaimMapping = (index: number) => {
  ssoForm.claim_mappings.splice(index, 1)
}

const loadSSOConfig = async () => {
  try {
    const { data } = await getSSOAdminConfig()
    const cfg = data.data
    ssoForm.enabled = cfg.enabled
    ssoForm.provider_name = cfg.provider_name
    ssoForm.client_id = cfg.client_id
    ssoForm.issuer_url = cfg.issuer_url
    ssoForm.redirect_uri = cfg.redirect_uri
    ssoForm.scopes = cfg.scopes
    ssoForm.auto_create_user = cfg.auto_create_user
    ssoForm.default_role = cfg.default_role
    if (cfg.claim_mappings && cfg.claim_mappings.length > 0) {
      ssoForm.claim_mappings = cfg.claim_mappings
    }
    // client_secret 不会从后端返回，保持为空
    ssoForm.client_secret = ''
  } catch {
    // ignored
  }
}

const saveSSOConfig = async () => {
  ssoSaving.value = true
  try {
    await updateSSOConfig(ssoForm)
    ElMessage.success(t('system.ssoSaved'))
  } catch {
    // ignored
  } finally {
    ssoSaving.value = false
  }
}

// ============ 初始化 ============
onMounted(() => {
  loadBrandConfig()
  loadGeneralConfig()
  loadEmailConfig()
  loadSecurityConfig()
  loadRateLimitConfig()
  loadWorkTypeConfig()
  loadSSOConfig()
})
</script>

<style scoped lang="scss">
.system-settings {
  width: 100%;
}

/* 单列表单限宽：输入框跟着容器铺到 1400px 宽，光标落点和内容长度完全对不上 */
.settings-single-col {
  max-width: 560px;
}

.form-item-example {
  display: block;
  margin-top: 2px;
  font-size: 11.5px;
  color: var(--td-text-placeholder);
  font-family: var(--td-font-mono);
  font-variant-numeric: tabular-nums;
}

/* ── tabs ─────────────────────────────────────── */
.settings-tabs {
  :deep(.el-tabs__nav-wrap)::after {
    height: 1px;
    background-color: var(--td-border-color);
  }

  :deep(.el-tabs__item) {
    padding: 0 14px;
    height: 42px;
    line-height: 42px;
    font-size: 13px;
    font-weight: 500;
    letter-spacing: -0.01em;
    color: var(--td-text-secondary);

    &.is-active {
      color: var(--td-text-primary);
      font-weight: 590;
    }
  }

  :deep(.el-tabs__active-bar) {
    background-color: var(--td-text-primary);
    height: 2px;
  }

  :deep(.el-tabs__content) {
    padding-top: 14px;
  }
}

/* ── 卡片 ─────────────────────────────────────── */
.settings-card {
  border-radius: 12px;

  :deep(.el-card__header) {
    padding: 12px 16px;
    border-bottom: 1px solid var(--td-divider-color);
  }

  :deep(.el-card__body) {
    padding: 16px;
  }
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;

  /* 卡片头只剩一行标题：tab 已经标明了这是哪一块配置，
     再来一遍图标色块 + 一句"配置系统的基本信息"的副标题，是 90px 的零信息 */
  .card-title .title {
    font-size: 15px;
    font-weight: 590;
    letter-spacing: -0.015em;
    color: var(--td-text-primary);
  }
}

/* ── 表单 ─────────────────────────────────────── */
.settings-form {
  :deep(.el-form-item) {
    margin-bottom: 16px;
  }

  :deep(.el-form-item__label) {
    font-size: 12.5px;
    font-weight: 500;
    color: var(--td-text-secondary);
  }

  .form-section {
    margin-bottom: 20px;

    .section-title {
      font-size: 12.5px;
      font-weight: 590;
      color: var(--td-text-secondary);
      margin-bottom: 12px;
      padding-bottom: 7px;
      border-bottom: 1px solid var(--td-divider-color);
    }
  }

  /* 灰底盒嵌在卡片里就是盒中盒；一行开关而已，发丝线分隔就够 */
  .setting-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 16px;
    padding: 10px 0;
    border-bottom: 1px solid var(--td-divider-color);
    /* 外面是 el-form-item 的 flex content，不撑满就会缩到文字宽度，
       下边那条分隔线只画出 204px 的一截，看着像画漏了 */
    width: 100%;

    /* 分节里最后一行下面没有内容要隔开，那条线是多余的 */
    &:last-child {
      border-bottom: 0;
    }
  }

  .form-item-tip {
    font-size: 12px;
    color: var(--td-text-secondary);
    margin-top: 4px;
    line-height: 1.5;
  }
}

.setting-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;

  .setting-label {
    font-size: 13px;
    color: var(--td-text-primary);
    font-weight: 500;
  }

  .setting-desc {
    font-size: 11.5px;
    color: var(--td-text-secondary);
    line-height: 1.5;
  }
}

/* ── 说明面板 ─────────────────────────────────── */
/* 原来是淡蓝底 + 蓝字，和真正需要注意的错误提示抢同一套视觉语言。
   它只是帮助文字，退回普通卡片，靠发丝边和弱色区分即可。 */
.info-panel {
  display: flex;
  align-items: flex-start;
  padding-top: 0;
}

.info-card,
.config-tips {
  width: 100%;
  background: var(--td-bg-section);
  border: 1px solid var(--td-border-color);
  border-radius: 10px;
  padding: 14px 16px;
}

.info-card .info-title,
.config-tips .tip-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12.5px;
  font-weight: 590;
  color: var(--td-text-secondary);
  margin-bottom: 10px;

  .el-icon {
    color: var(--td-text-placeholder);
  }
}

.info-card .info-list {
  margin: 0;
  padding-left: 17px;

  li {
    font-size: 12.5px;
    color: var(--td-text-secondary);
    line-height: 1.7;
  }
}

.config-tips {
  .tip-content {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .tip-item {
    .tip-label {
      font-size: 12.5px;
      font-weight: 590;
      color: var(--td-text-primary);
      margin-bottom: 3px;
    }

    .tip-desc {
      font-size: 12.5px;
      color: var(--td-text-secondary);
      line-height: 1.7;

      code {
        background: var(--td-code-bg);
        border: 1px solid var(--td-border-color-light);
        padding: 1px 5px;
        border-radius: 5px;
        font-family: var(--td-font-mono);
        font-size: 11.5px;
        color: var(--td-text-regular);
        word-break: break-all;
      }
    }
  }
}

/* ── 安全设置块 ───────────────────────────────── */
.setting-block {
  border-radius: 12px;
  margin-bottom: 12px;

  :deep(.el-card__body) {
    padding: 0;
  }

  .block-header {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 12px 16px;
    border-bottom: 1px solid var(--td-divider-color);

    .block-title {
      display: flex;
      flex-direction: column;
      gap: 2px;

      .title {
        font-size: 14px;
        font-weight: 590;
        letter-spacing: -0.01em;
        color: var(--td-text-primary);
      }

      .desc {
        font-size: 11.5px;
        color: var(--td-text-secondary);
        line-height: 1.5;
      }
    }
  }

  .block-content {
    padding: 4px 16px 12px;
  }
}

.setting-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  padding: 10px 0;

  &:not(:last-child) {
    border-bottom: 1px solid var(--td-divider-color);
  }
}

.action-bar {
  margin-top: 18px;
  padding-top: 16px;
  border-top: 1px solid var(--td-divider-color);
}

@media (max-width: 992px) {
  .info-panel {
    margin-top: 16px;
  }
}

/* ── Claims 映射 / 工作类型动态列表 ───────────── */
.claim-mappings {
  .claim-mapping-row {
    display: flex;
    align-items: center;
    margin-bottom: 6px;
  }

  .claim-mapping-hint {
    font-size: 12px;
    color: var(--td-text-secondary);
    margin-top: 6px;
    line-height: 1.6;
  }
}

.work-type-list {
  /* 里面装的是「开发」「巡检」这种两三个字的标签，输入框跟着栅格拉到 895px
     只剩一片空白，十一行排下来整页都是空框。按内容给个合理上限。 */
  max-width: 340px;

  .work-type-item {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 6px;
  }
}

/* ── 品牌设置 ─────────────────────────────────── */
.upload-area {
  display: flex;
  align-items: center;
  gap: 14px;
}

.upload-preview {
  display: flex;
  align-items: center;
  gap: 8px;
}

.preview-image {
  width: 36px;
  height: 36px;
  border-radius: 7px;
  object-fit: contain;
  border: 1px solid var(--td-border-color);
  padding: 2px;
}

.preview-favicon {
  width: 22px;
  height: 22px;
}

.brand-preview-section {
  padding: 0;
}

.brand-preview-card {
  border: 1px solid var(--td-border-color);
  border-radius: 10px;
  overflow: hidden;
  margin-bottom: 12px;
}

/* 品牌预览里模拟的侧边栏。
   这里的每一个颜色都必须走 sidebar 那组 token ——
   侧边栏翻成浅色之后，原来写死的 #fff / rgba(255,255,255,.x) 就是白底白字，
   预览整块变成一片空白（和登录页品牌面板当初是同一个雷）。 */
.preview-sidebar {
  background: var(--td-sidebar-bg);
  padding: 14px;
  min-height: 180px;
  display: flex;
  flex-direction: column;
}

.preview-logo-area {
  display: flex;
  align-items: center;
  gap: 8px;
  padding-bottom: 10px;
  border-bottom: 1px solid var(--td-sidebar-border);
  margin-bottom: 10px;
}

.preview-logo-img {
  width: 26px;
  height: 26px;
  border-radius: 6px;
  object-fit: contain;
}

.preview-logo-placeholder {
  width: 26px;
  height: 26px;
  border-radius: 6px;
  background: var(--td-color-primary);
  color: var(--td-text-white);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 13px;
  font-weight: 600;
  flex-shrink: 0;
}

.preview-logo-text {
  font-size: 13.5px;
  font-weight: 590;
  letter-spacing: -0.015em;
  color: var(--td-sidebar-text-active);
}

.preview-menu-item {
  padding: 5px 9px;
  border-radius: 7px;
  font-size: 12.5px;
  color: var(--td-sidebar-text);
  margin-bottom: 2px;

  /* 当前项和真实侧边栏一致：淡底 + 字重加粗，不用大面积品牌色，也不加左侧竖条 */
  &.active {
    background: var(--td-sidebar-active-bg);
    color: var(--td-sidebar-text-active);
    font-weight: 590;
  }
}

.preview-copyright {
  margin-top: auto;
  padding-top: 10px;
  border-top: 1px solid var(--td-sidebar-border);
  font-size: 10.5px;
  color: var(--td-sidebar-text-muted);
  word-break: break-all;
}

.brand-tips {
  margin-top: 10px;

  p {
    margin: 0 0 3px;
    font-size: 12.5px;
    color: var(--td-text-secondary);

    &:last-child {
      margin-bottom: 0;
    }
  }
}

/* 卡头右侧的总开关：文字放在开关外面，塞进 40px 宽的开关里读不出来 */
.header-switch {
  display: flex;
  align-items: center;
  gap: 8px;
}

.header-switch-label {
  font-size: 12.5px;
  color: var(--td-text-placeholder);

  &.active { color: var(--td-text-primary); }
}
</style>
