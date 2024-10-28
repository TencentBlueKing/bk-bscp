<template>
  <bk-form ref="formRef" form-type="vertical" :model="localVal" :rules="rules">
    <bk-form-item :label="t('配置文件名')" property="fileAP" :required="true">
      <bk-input
        v-model="localVal.fileAP"
        :placeholder="t('请输入配置文件的完整路径和文件名，例如：/etc/nginx/nginx.conf')"
        :disabled="isEdit"
        @input="handleFileAPInput" />
    </bk-form-item>
    <bk-form-item :label="t('配置文件描述')" property="memo">
      <bk-input
        v-model="localVal.memo"
        type="textarea"
        :maxlength="200"
        :placeholder="t('请输入')"
        :resize="true"
        @input="change" />
    </bk-form-item>
    <bk-form-item :label="t('配置文件格式')">
      <bk-radio-group v-model="localVal.file_type" :required="true" @change="change">
        <bk-radio v-for="typeItem in CONFIG_FILE_TYPE" :key="typeItem.id" :label="typeItem.id" :disabled="isEdit">
          {{ typeItem.name }}
        </bk-radio>
      </bk-radio-group>
    </bk-form-item>
    <UserSetting
      :bk-biz-id="props.bkBizId"
      :id="props.id"
      :is-tpl="props.isTpl"
      :form="privilegeForm as IConfigPrivilegeForm"
      @change="handlePrivilegeChange" />
    <div v-if="isWindowsAgent" class="user-tips">
      <info-line class="icon" />
      <span>{{ t('对于Windows客户端，以上文件权限、用户及用户组设置不生效，可在后置脚本中处理文件权限') }}</span>
    </div>
    <bk-form-item class="config-content" v-if="localVal.file_type === 'binary'" :label="t('配置内容')" :required="true">
      <bk-upload
        class="config-uploader"
        url=""
        theme="button"
        :tip="t('文件大小{size}M以内', { size: props.fileSizeLimit })"
        :size="100000"
        :multiple="false"
        :custom-request="handleFileUpload">
      </bk-upload>
      <bk-loading
        v-if="uploadFile"
        mode="spin"
        theme="primary"
        :opacity="0.6"
        size="mini"
        :title="t('文件下载中，请稍后')"
        :loading="fileDownloading"
        class="file-down-loading">
        <div :class="['file-wrapper', { 'upload-fail': uploadFile.status === 'fail' }]" @click="handleDownloadFile">
          <TextFill class="file-icon" />
          <div class="file-content">
            <div class="name" :title="uploadFile?.file.name">{{ uploadFile?.file.name }}</div>
            <div v-if="uploadFile.status === 'checking'" class="check-status">
              <Spinner class="spinner-icon" /> {{ $t('文件上传准备中，请稍候…') }}
            </div>
            <div v-else-if="uploadProgress.status === 'uploading'">
              <bk-progress
                :percent="uploadProgress.percent"
                :theme="uploadFile.status === 'fail' ? 'danger' : 'primary'"
                size="small"
                :show-text="false" />
            </div>
            <div v-else :class="[uploadFile.status === 'success' ? 'success-text' : 'error-text', 'status-icon-area']">
              <Done v-if="uploadFile.status === 'success'" class="success-icon" />
              <Error v-if="uploadFile.status === 'fail'" class="error-icon" />
              <span :class="[uploadFile.status === 'success' ? 'success-text' : 'error-text']">
                {{ uploadFile.status === 'success' ? t('上传成功') : `${t('上传失败')} ${uploadFile.errorMessage}` }}
                <span v-if="uploadFile.status === 'success' && uploadFile.isExist">
                  {{ $t('( 后台已存在此文件，上传快速完成 )') }}
                </span>
              </span>
            </div>
          </div>
          <span class="size">
            ({{ byteUnitConverse(uploadFile.file.size) }})
            <span v-if="uploadProgress.status === 'uploading'">{{ `${uploadProgress.percent}%` }}</span>
          </span>
        </div>
      </bk-loading>
    </bk-form-item>
    <bk-form-item class="config-content" v-else>
      <template #label>
        <div class="config-content-label">
          <span>{{ t('配置内容') }}</span>
          <info v-bk-tooltips="{ content: t('tips.createConfig'), placement: 'top' }" fill="#3a84ff" class="icon" />
        </div>
      </template>
      <ConfigContentEditor
        :content="stringContent"
        :editable="true"
        :variables="props.variables"
        :size-limit="props.fileSizeLimit"
        @change="handleStringContentChange" />
    </bk-form-item>
  </bk-form>
</template>
<script setup lang="ts">
  import { ref, watch, onMounted, computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import SHA256 from 'crypto-js/sha256';
  import WordArray from 'crypto-js/lib-typedarrays';
  import CryptoJS from 'crypto-js';
  import { TextFill, Done, Info, Error, Spinner, InfoLine } from 'bkui-vue/lib/icon';
  import BkMessage from 'bkui-vue/lib/message';
  import { cloneDeep } from 'lodash';
  import {
    IConfigEditParams,
    IFileConfigContentSummary,
    IConfigPrivilegeForm,
  } from '../../../../../../../../types/config';
  import { IVariableEditParams } from '../../../../../../../../types/variable';
  import {
    updateConfigContent,
    downloadConfigContent,
    getConfigUploadFileIsExist,
  } from '../../../../../../../api/config';
  import {
    downloadTemplateContent,
    updateTemplateContent,
    getTemplateUploadFileIsExist,
  } from '../../../../../../../api/template';
  import { stringLengthInBytes, byteUnitConverse } from '../../../../../../../utils/index';
  import { fileDownload } from '../../../../../../../utils/file';
  import { CONFIG_FILE_TYPE } from '../../../../../../../constants/config';
  import ConfigContentEditor from '../../components/config-content-editor.vue';
  import UserSetting from '../../components/user-setting.vue';

  interface IUploadFile {
    file: any;
    status: string;
    isExist: boolean;
    errorMessage?: string;
  }

  const { t } = useI18n();

  const props = withDefaults(
    defineProps<{
      config: IConfigEditParams;
      isEdit: boolean;
      content?: string | IFileConfigContentSummary;
      variables?: IVariableEditParams[];
      bkBizId: string;
      id: number; // 服务ID或者模板空间ID
      fileUploading?: boolean;
      fileSizeLimit?: number;
      isTpl?: boolean; // 是否未模板配置文件，非模板配置文件和模板配置文件的上传、下载接口参数有差异
    }>(),
    {
      isEdit: false,
      fileSizeLimit: 100,
    },
  );

  const emits = defineEmits(['change', 'update:fileUploading']);
  const localVal = ref({ ...props.config, fileAP: '' });
  const stringContent = ref('');
  const fileContent = ref<IFileConfigContentSummary | File>();
  const uploadFileSignature = ref(''); // 新上传文件的sha256
  const isFileChanged = ref(false); // 标识文件是否被修改，编辑配置文件时若文件未修改，不重新上传文件
  const formRef = ref();
  const uploadProgress = ref({
    percent: 0,
    status: '',
  });
  const fileDownloading = ref(false);
  const uploadFile = ref<IUploadFile>();
  const isWindowsAgent = ref(false); // 是否为windows用户
  const rules = {
    // 配置文件名校验规则，path+filename
    fileAP: [
      {
        validator: (val: string) => /^\/(?:[^/]+\/)*[^/]+$/.test(val),
        message: t('无效的路径,路径不符合Unix文件路径格式规范'),
        trigger: 'change',
      },
    ],
    privilege: [
      {
        required: true,
        validator: () => {
          const type = typeof localVal.value.privilege;
          return type === 'number' || (type === 'string' && localVal.value.privilege!.length > 0);
        },
        message: t('文件权限 不能为空'),
        trigger: 'change',
      },
      {
        validator: () => {
          const privilege = parseInt(localVal.value.privilege![0], 10);
          return privilege >= 4;
        },
        message: t('文件own必须有读取权限'),
        trigger: 'blur',
      },
    ],
    memo: [
      {
        validator: (value: string) => value.length <= 200,
        message: t('最大长度200个字符'),
      },
    ],
    revision_name: [
      {
        validator: (value: string) => value.length <= 128,
        message: t('最大长度128个字符'),
      },
      {
        validator: (value: string) => {
          if (value.length > 0) {
            return /^[\u4e00-\u9fa5a-zA-Z0-9][\u4e00-\u9fa5a-zA-Z0-9_-]*[\u4e00-\u9fa5a-zA-Z0-9]?$/.test(value);
          }
          return true;
        },
        message: t('仅允许使用中文、英文、数字、下划线、中划线，且必须以中文、英文、数字开头和结尾'),
      },
    ],
  };

  const privilegeForm = computed(() => {
    const { privilege, user, user_group, uid, gid } = localVal.value;
    return {
      privilege,
      user,
      user_group,
      uid,
      gid,
    };
  });

  watch(
    () => props.config,
    () => {
      const { path, name } = props.config;
      if (!path) return;
      localVal.value.fileAP = path.endsWith('/') ? `${path}${name}` : `${path}/${name}`;
    },
    { immediate: true, deep: true },
  );

  onMounted(() => {
    if (props.config.file_type === 'binary') {
      fileContent.value = cloneDeep(props.content as IFileConfigContentSummary);
      if (props.isEdit) {
        if (fileContent.value.signature) {
          uploadFile.value = {
            file: { ...fileContent.value },
            status: 'success',
            isExist: false,
          };
          uploadFileSignature.value = fileContent.value.signature;
        }
      }
    } else {
      stringContent.value = props.content as string;
    }
    isWindowsAgent.value = navigator.userAgent.indexOf('Windows') !== -1;
  });

  const handleStringContentChange = (val: string) => {
    stringContent.value = val;
    change();
  };

  // 选择文件后上传
  const handleFileUpload = async (option: { file: File }) => {
    emits('update:fileUploading', true);
    fileContent.value = option.file;
    uploadFile.value = {
      file: option.file,
      status: 'checking',
      isExist: false,
    };
    const fileSize = option.file.size / 1024 / 1024;
    if (fileSize > props.fileSizeLimit) {
      uploadFile.value!.status = 'fail';
      uploadFile.value.errorMessage = t('请确保文件大小不超过 {n} MB', { n: props.fileSizeLimit });
      return;
    }
    isFileChanged.value = true;
    if (localVal.value.fileAP === '') {
      localVal.value.fileAP = `/${option.file.name}`;
    }
    // 文件存在 无需重复上传
    const res = await checkFileExist();
    if (res.exists) {
      uploadFile.value.status = 'success';
      uploadFile.value.isExist = true;
      fileContent.value = {
        name: option.file.name,
        signature: res.metadata.sha256,
        size: res.metadata.byte_size,
      };
      change();
      emits('update:fileUploading', false);
      return Promise.resolve();
    }
    uploadFile.value.status = 'uploading';
    uploadFile.value.isExist = false;
    return new Promise((resolve, reject) => {
      uploadContent()
        .then((res) => {
          uploadFile.value!.status = 'success';
          fileContent.value = {
            name: option.file.name,
            signature: res.sha256,
            size: res.byte_size,
          };
          change();
          resolve(res);
        })
        .catch((err) => {
          console.error(err);
          uploadFile.value!.status = 'fail';
          uploadFile.value!.errorMessage = '';
          reject(err);
        })
        .finally(() => {
          emits('update:fileUploading', false);
          uploadProgress.value.status = 'success';
          uploadProgress.value.percent = 0;
        });
    });
  };

  // 上传配置内容
  const uploadContent = async () => {
    uploadProgress.value.status = 'uploading';
    if (props.isTpl) {
      return updateTemplateContent(
        props.bkBizId,
        props.id,
        fileContent.value as File,
        uploadFileSignature.value,
        (progress: number) => {
          uploadProgress.value.percent = progress;
        },
      );
    }
    return updateConfigContent(
      props.bkBizId,
      props.id,
      fileContent.value as File,
      uploadFileSignature.value,
      (progress: number) => {
        uploadProgress.value.percent = progress;
      },
    );
  };

  // 判断上传的文件是否存在
  const checkFileExist = async () => {
    const signature = await getSignature();
    uploadFileSignature.value = signature;
    if (props.isTpl) {
      return getTemplateUploadFileIsExist(props.bkBizId, props.id, signature);
    }
    return getConfigUploadFileIsExist(props.bkBizId, props.id, signature);
  };

  // 生成文件或文本的sha256
  const getSignature = async () => {
    if (localVal.value.file_type === 'binary') {
      const CHUNK_SIZE = 1024 * 1024; // 1MB
      // 初始化第一个切片的处理
      let start = 0;
      let end = Math.min(CHUNK_SIZE, fileContent.value!.size as number);
      if (isFileChanged.value) {
        return new Promise((resolve) => {
          const reader = new FileReader();
          const hash = CryptoJS.algo.SHA256.create();
          const processChunk = () => {
            // @ts-ignore
            const slice = fileContent.value.slice(start, end);
            reader.readAsArrayBuffer(slice);
          };
          reader.onload = function () {
            const wordArray = WordArray.create(reader.result);
            hash.update(wordArray);
            if (end < (fileContent.value!.size as number)) {
              start += CHUNK_SIZE;
              end = Math.min(start + CHUNK_SIZE, fileContent.value!.size as number);
              processChunk();
            } else {
              const sha256Hash = hash.finalize();
              resolve(sha256Hash.toString());
            }
          };
          // 开始处理第一个切片
          processChunk();
        });
      }
      return (fileContent.value as IFileConfigContentSummary).signature;
    }
    if (!stringContent.value.endsWith('\n')) stringContent.value += '\n';
    return SHA256(stringContent.value).toString();
  };

  // 下载已上传文件
  const handleDownloadFile = async () => {
    if (uploadProgress.value.status === 'uploading') return;
    try {
      fileDownloading.value = true;
      const { signature, name } = fileContent.value as IFileConfigContentSummary;
      const fileSignature = signature || uploadFileSignature.value;
      const getContent = props.isTpl ? downloadTemplateContent : downloadConfigContent;
      const res = await getContent(props.bkBizId, props.id, fileSignature, true);
      fileDownload(res, name);
    } catch (error) {
      console.error(error);
    } finally {
      fileDownloading.value = false;
    }
  };

  const validate = async () => {
    await formRef.value.validate();
    if (localVal.value.file_type === 'binary') {
      if (!uploadFile.value) {
        BkMessage({ theme: 'error', message: t('请上传文件') });
        return false;
      }
      if (uploadFile.value.status === 'fail') {
        BkMessage({ theme: 'error', message: t('文件上传失败，请重新上传文件') });
        return false;
      }
    } else if (localVal.value.file_type === 'text') {
      if (stringLengthInBytes(stringContent.value) > 1024 * 1024 * props.fileSizeLimit) {
        BkMessage({ theme: 'error', message: t('配置内容不能超过{size}M', { size: props.fileSizeLimit }) });
        return false;
      }
    }
    return true;
  };

  const change = () => {
    const content = localVal.value.file_type === 'binary' ? fileContent.value : stringContent.value;
    const { fileAP } = localVal.value;
    const lastSlashIndex = fileAP.lastIndexOf('/');
    localVal.value.name = fileAP.slice(lastSlashIndex + 1);
    localVal.value.path = fileAP.slice(0, lastSlashIndex + 1);
    emits('change', localVal.value, content);
  };

  const handleFileAPInput = () => {
    // 用户输入文件名 补全路径
    if (localVal.value.fileAP && !localVal.value.fileAP.startsWith('/')) {
      localVal.value.fileAP = `/${localVal.value.fileAP}`;
    }
    change();
  };

  // 权限内容修改
  const handlePrivilegeChange = (privilegeForm: IConfigPrivilegeForm) => {
    localVal.value = { ...localVal.value, ...privilegeForm };
    change();
  };

  defineExpose({
    getSignature: () => {
      if (localVal.value.file_type === 'binary') {
        return uploadFileSignature.value;
      }
      return getSignature();
    },
    validate,
  });
</script>
<style lang="scss" scoped>
  .user-tips {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 8px;
    font-size: 12px;
    color: #63656e;
    .icon {
      font-size: 14px;
      color: #979ba5;
    }
  }
  .config-content {
    margin-top: 24px;
  }
  :deep(.config-uploader) {
    .bk-upload-list {
      display: none;
    }
  }
  .file-wrapper {
    margin: 8px 0;
    position: relative;
    display: flex;
    align-items: center;
    color: #979ba5;
    font-size: 12px;
    line-height: 12px;
    width: 100%;
    border: 1px solid #c4c6cc;
    padding: 10px;
    cursor: pointer;
    &.upload-fail {
      border-color: #ff5656;
      background: #fedddc66;
    }
    &:hover .name {
      color: #3a84ff;
    }
    .file-content {
      width: 400px;
      line-height: 20px;
      .spinner-icon {
        font-size: 14px;
        color: #3a84ff;
      }
    }
    .status-icon-area {
      display: flex;
      align-items: center;
      &.success-text {
        color: #2dcb56;
      }
      &.error-text {
        color: #ea3636;
      }
      .success-icon {
        font-size: 20px;
      }
      .error-icon {
        font-size: 14px;
      }
    }
    .file-icon {
      margin: 0 6px 0 0;
      font-size: 32px;
    }
    .name {
      max-width: 360px;
      white-space: nowrap;
      text-overflow: ellipsis;
      overflow: hidden;
      line-height: normal;
    }
    .size {
      position: absolute;
      right: 10px;
      top: 50%;
      transform: translateY(-50%);
    }
  }
  .config-content-label {
    display: flex;
    align-items: center;
    span {
      margin-right: 5px;
    }
    .icon {
      font-size: 14px;
    }
  }
  .file-down-loading {
    width: 100%;
    :deep(.bk-loading-indicator) {
      align-items: center;
      flex-direction: row;
      .bk-loading-title {
        margin-top: 0px;
        margin-left: 8px;
        color: #979ba5;
        font-size: 12px;
      }
    }
  }
  .type-group {
    :deep(.bk-radio-label) {
      font-size: 12px;
    }
  }
</style>
<style lang="scss">
  .privilege-select-popover.bk-popover {
    padding: 0;
    .bk-pop2-arrow {
      border-left: 1px solid #dcdee5;
      border-top: 1px solid #dcdee5;
    }
  }
  .privilege-tips-wrap {
    border: 1px solid #dcdee5;
    .bk-pop2-arrow {
      border-right: 1px solid #dcdee5;
      border-bottom: 1px solid #dcdee5;
    }
  }
</style>
