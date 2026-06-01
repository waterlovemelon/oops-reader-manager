import { UploadOutlined, SearchOutlined, ReloadOutlined, StopOutlined, ClockCircleOutlined, SyncOutlined, CheckCircleOutlined, CloseCircleOutlined, EyeOutlined } from '@ant-design/icons';
import { Button, Card, Descriptions, Input, Modal, Select, Space, Table, Tag, Typography, Upload, message, Form, Progress, Tooltip } from 'antd';
import type { UploadProps } from 'antd';
import { useCallback, useEffect, useRef, useState } from 'react';
import {
  CatalogBook, ImportJob, listBooks, updateBook, updateBookStatus,
  createImportJob, getImportJob, retryImportJob, cancelImportJob, fetchBookCover,
} from '../api/catalog';

const STATUS_COLORS: Record<string, string> = {
  draft: 'gold',
  active: 'green',
  hidden: 'orange',
  deleted: 'red',
};

const JOB_STATUS_CONFIG: Record<string, { color: string; icon: React.ReactNode; label: string }> = {
  queued: { color: 'blue', icon: <ClockCircleOutlined />, label: '排队中' },
  processing: { color: 'processing', icon: <SyncOutlined spin />, label: '导入中' },
  succeeded: { color: 'green', icon: <CheckCircleOutlined />, label: '已完成' },
  failed: { color: 'red', icon: <CloseCircleOutlined />, label: '失败' },
  canceled: { color: 'default', icon: <StopOutlined />, label: '已取消' },
};

const STAGE_LABELS: Record<string, string> = {
  uploaded: '已上传',
  hashing: '计算哈希',
  duplicate_check: '检查重复',
  parsing_metadata: '解析元数据',
  extracting_cover: '提取封面',
  splitting_chapters: '分割章节',
  writing_storage: '写入存储',
  creating_book: '创建书籍',
  recording_audit: '记录审计',
  finished: '完成',
};

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return bytes + ' B';
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
  return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
}

export function BooksPage() {
  const [books, setBooks] = useState<CatalogBook[]>([]);
  const [loading, setLoading] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState('');
  const [statusFilter, setStatusFilter] = useState<string>('');
  const [editBook, setEditBook] = useState<CatalogBook | null>(null);
  const [editForm] = Form.useForm();
  const [detailBook, setDetailBook] = useState<CatalogBook | null>(null);
  const [coverUrl, setCoverUrl] = useState<string | null>(null);

  // Import jobs state
  const [importJobs, setImportJobs] = useState<ImportJob[]>([]);
  const pollTimers = useRef<Map<string, ReturnType<typeof setTimeout>>>(new Map());

  const fetchBooks = () => {
    setLoading(true);
    listBooks(search || undefined, statusFilter || undefined, page)
      .then((res) => {
        setBooks(res.data);
        setTotal(res.pagination.total);
      })
      .catch(() => message.error('加载失败'))
      .finally(() => setLoading(false));
  };

  useEffect(() => { fetchBooks(); }, [page, statusFilter]);

  const handleSearch = () => {
    setPage(1);
    fetchBooks();
  };

  // Poll a single import job until it reaches a terminal state.
  const pollJob = useCallback((jobID: string) => {
    const timer = setTimeout(async () => {
      try {
        const res = await getImportJob(jobID);
        const job = res.data;

        setImportJobs((prev) => {
          const idx = prev.findIndex((j) => j.job_id === jobID);
          if (idx >= 0) {
            const next = [...prev];
            next[idx] = job;
            return next;
          }
          return prev;
        });

        if (job.status === 'processing' || job.status === 'queued') {
          pollJob(jobID); // continue polling
        } else {
          // Terminal state
          pollTimers.current.delete(jobID);
          if (job.status === 'succeeded') {
            message.success(`《${job.original_filename}》导入成功`);
            fetchBooks(); // refresh book list
          } else if (job.status === 'failed') {
            message.error(`导入失败: ${job.error_message ?? '未知错误'}`);
          }
        }
      } catch {
        pollTimers.current.delete(jobID);
      }
    }, 1500);
    pollTimers.current.set(jobID, timer);
  }, []);

  // Cleanup polling on unmount.
  useEffect(() => {
    const timers = pollTimers.current;
    return () => {
      timers.forEach((t) => clearTimeout(t));
      timers.clear();
    };
  }, []);

  const handleUpload: UploadProps['beforeUpload'] = async (file) => {
    setUploading(true);
    try {
      const result = await createImportJob(file);
      message.success('文件已上传，开始导入...');

      // Add job to tracking list.
      const placeholder: ImportJob = {
        job_id: result.job_id,
        admin_username: '',
        original_filename: file.name,
        format: file.name.endsWith('.epub') ? 'epub' : 'txt',
        content_sha1: '',
        file_size: file.size,
        status: 'queued',
        stage: result.stage,
        attempt_count: 0,
        max_attempts: 3,
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
      };
      setImportJobs((prev) => [placeholder, ...prev]);

      // Start polling.
      pollJob(result.job_id);
    } catch (error) {
      message.error(error instanceof Error ? error.message : '上传失败');
    } finally {
      setUploading(false);
    }
    return false;
  };

  const handleRetry = async (jobID: string) => {
    try {
      await retryImportJob(jobID);
      message.success('已重新排队');
      setImportJobs((prev) =>
        prev.map((j) => j.job_id === jobID ? { ...j, status: 'queued' as const, error_message: undefined } : j)
      );
      pollJob(jobID);
    } catch (error) {
      message.error(error instanceof Error ? error.message : '重试失败');
    }
  };

  const handleCancel = async (jobID: string) => {
    try {
      await cancelImportJob(jobID);
      message.success('已取消');
      setImportJobs((prev) =>
        prev.map((j) => j.job_id === jobID ? { ...j, status: 'canceled' as const } : j)
      );
    } catch (error) {
      message.error(error instanceof Error ? error.message : '取消失败');
    }
  };

  // Remove completed/failed/canceled jobs from the active list.
  const dismissJob = (jobID: string) => {
    setImportJobs((prev) => prev.filter((j) => j.job_id !== jobID));
  };

  const handleStatusChange = async (bookKey: string, newStatus: string) => {
    try {
      await updateBookStatus(bookKey, newStatus);
      message.success('状态已更新');
      fetchBooks();
    } catch (error) {
      message.error(error instanceof Error ? error.message : '操作失败');
    }
  };

  const handleEdit = (book: CatalogBook) => {
    setEditBook(book);
    editForm.setFieldsValue({
      title: book.title,
      author: book.author,
      description: book.description,
      language: book.language,
    });
  };

  const handleEditSubmit = async () => {
    if (!editBook) return;
    try {
      const values = await editForm.validateFields();
      await updateBook(editBook.book_key, values);
      message.success('更新成功');
      setEditBook(null);
      fetchBooks();
    } catch (error) {
      message.error(error instanceof Error ? error.message : '更新失败');
    }
  };

  const handleDetail = async (book: CatalogBook) => {
    setDetailBook(book);
    if (coverUrl) { URL.revokeObjectURL(coverUrl); setCoverUrl(null); }
    if (book.format === 'epub') {
      const url = await fetchBookCover(book.book_key);
      setCoverUrl(url);
    }
  };

  // Cleanup blob URL on unmount.
  useEffect(() => {
    return () => { if (coverUrl) URL.revokeObjectURL(coverUrl); };
  }, [coverUrl]);

  return (
    <>
      {/* Active import jobs */}
      {importJobs.length > 0 && (
        <Card title="导入任务" size="small" style={{ marginBottom: 16 }}>
          <Table
            rowKey="job_id"
            dataSource={importJobs}
            pagination={false}
            size="small"
            columns={[
              { title: '文件名', dataIndex: 'original_filename', ellipsis: true },
              { title: '格式', dataIndex: 'format', width: 70, render: (v: string) => <Tag>{v}</Tag> },
              {
                title: '状态', dataIndex: 'status', width: 100,
                render: (v: string) => {
                  const cfg = JOB_STATUS_CONFIG[v] ?? { color: 'default', icon: null, label: v };
                  return <Tag color={cfg.color} icon={cfg.icon}>{cfg.label}</Tag>;
                },
              },
              {
                title: '进度', dataIndex: 'stage', width: 150,
                render: (_: string, record: ImportJob) => {
                  if (record.status === 'succeeded') return <Progress percent={100} size="small" />;
                  if (record.status === 'failed') return <span style={{ color: '#ff4d4f', fontSize: 12 }}>{record.error_message}</span>;
                  if (record.status === 'canceled') return <span style={{ color: '#999', fontSize: 12 }}>已取消</span>;
                  return (
                    <Tooltip title={STAGE_LABELS[record.stage] ?? record.stage}>
                      <Progress
                        percent={record.progress_percent ?? (record.status === 'queued' ? 0 : 50)}
                        size="small"
                        status={record.status === 'processing' ? 'active' : 'normal'}
                      />
                    </Tooltip>
                  );
                },
              },
              {
                title: '操作', width: 150,
                render: (_: unknown, record: ImportJob) => (
                  <Space size="small">
                    {record.status === 'failed' && (
                      <Button size="small" icon={<ReloadOutlined />} onClick={() => handleRetry(record.job_id)}>重试</Button>
                    )}
                    {record.status === 'queued' && (
                      <Button size="small" icon={<StopOutlined />} onClick={() => handleCancel(record.job_id)}>取消</Button>
                    )}
                    {(record.status === 'succeeded' || record.status === 'failed' || record.status === 'canceled') && (
                      <Button size="small" type="link" onClick={() => dismissJob(record.job_id)}>关闭</Button>
                    )}
                    {record.status === 'succeeded' && record.book_key && (
                      <Button size="small" type="link" onClick={() => {
                        // Navigate could be added here
                        message.info(`书籍 ID: ${record.book_key}`);
                      }}>查看书籍</Button>
                    )}
                  </Space>
                ),
              },
            ]}
          />
        </Card>
      )}

      <Card
        title="在线书籍"
        extra={
          <Upload accept=".epub,.txt" showUploadList={false} beforeUpload={handleUpload}>
            <Button icon={<UploadOutlined />} loading={uploading}>上传 EPUB/TXT</Button>
          </Upload>
        }
      >
        <Space style={{ marginBottom: 16 }}>
          <Input
            placeholder="搜索书名/作者"
            prefix={<SearchOutlined />}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            onPressEnter={handleSearch}
            style={{ width: 240 }}
            allowClear
          />
          <Select
            placeholder="状态筛选"
            value={statusFilter || undefined}
            onChange={(v) => { setStatusFilter(v ?? ''); setPage(1); }}
            allowClear
            style={{ width: 120 }}
            options={[
              { value: 'draft', label: '草稿' },
              { value: 'active', label: '已发布' },
              { value: 'hidden', label: '已隐藏' },
            ]}
          />
          <Button onClick={handleSearch}>搜索</Button>
        </Space>
        <Table
          rowKey="book_key"
          loading={loading}
          dataSource={books}
          pagination={{
            current: page,
            pageSize: 20,
            total,
            onChange: setPage,
            showTotal: (t) => `共 ${t} 本`,
          }}
          columns={[
            { title: '书名', dataIndex: 'title', ellipsis: true },
            { title: '作者', dataIndex: 'author', ellipsis: true },
            { title: '格式', dataIndex: 'format', width: 80, render: (v: string) => <Tag>{v}</Tag> },
            { title: '章节', dataIndex: 'chapter_count', width: 70 },
            {
              title: '状态', dataIndex: 'status', width: 100,
              render: (v: string) => <Tag color={STATUS_COLORS[v] ?? 'default'}>{v}</Tag>,
            },
            { title: '上传时间', dataIndex: 'uploaded_at', width: 170 },
            {
              title: '操作', width: 260,
              render: (_: unknown, record: CatalogBook) => (
                <Space size="small">
                  <Button size="small" icon={<EyeOutlined />} onClick={() => handleDetail(record)}>详情</Button>
                  <Button size="small" onClick={() => handleEdit(record)}>编辑</Button>
                  {record.status === 'draft' && (
                    <Button size="small" type="primary" onClick={() => handleStatusChange(record.book_key, 'active')}>发布</Button>
                  )}
                  {record.status === 'active' && (
                    <Button size="small" onClick={() => handleStatusChange(record.book_key, 'hidden')}>隐藏</Button>
                  )}
                  {record.status === 'hidden' && (
                    <Button size="small" type="primary" onClick={() => handleStatusChange(record.book_key, 'active')}>恢复</Button>
                  )}
                  {record.status !== 'deleted' && (
                    <Button size="small" danger onClick={() => {
                      Modal.confirm({
                        title: '确认删除？',
                        content: `确定要删除《${record.title}》吗？`,
                        onOk: () => handleStatusChange(record.book_key, 'deleted'),
                      });
                    }}>删除</Button>
                  )}
                </Space>
              ),
            },
          ]}
        />
      </Card>

      <Modal
        title="编辑书籍"
        open={!!editBook}
        onCancel={() => setEditBook(null)}
        onOk={handleEditSubmit}
        destroyOnHidden
      >
        <Form form={editForm} layout="vertical">
          <Form.Item name="title" label="书名" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="author" label="作者">
            <Input />
          </Form.Item>
          <Form.Item name="description" label="简介">
            <Input.TextArea rows={3} />
          </Form.Item>
          <Form.Item name="language" label="语言">
            <Input />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="书籍详情"
        open={!!detailBook}
        onCancel={() => { setDetailBook(null); if (coverUrl) { URL.revokeObjectURL(coverUrl); setCoverUrl(null); } }}
        footer={null}
        width={640}
        destroyOnHidden
      >
        {detailBook && (
          <div style={{ display: 'flex', gap: 24 }}>
            <div style={{ flexShrink: 0, width: 180 }}>
              {detailBook.format === 'epub' ? (
                coverUrl ? (
                  <img src={coverUrl} alt={detailBook.title} style={{ width: '100%', borderRadius: 4 }} />
                ) : (
                  <div style={{ width: 180, height: 240, background: '#f0f0f0', display: 'flex', alignItems: 'center', justifyContent: 'center', borderRadius: 4 }}>
                    加载封面中...
                  </div>
                )
              ) : (
                <div style={{ width: 180, height: 240, background: '#f0f0f0', display: 'flex', alignItems: 'center', justifyContent: 'center', borderRadius: 4, fontSize: 24, color: '#999' }}>
                  {detailBook.format.toUpperCase()}
                </div>
              )}
            </div>
            <Descriptions column={1} size="small" style={{ flex: 1 }}>
              <Descriptions.Item label="书名">{detailBook.title}</Descriptions.Item>
              <Descriptions.Item label="作者">{detailBook.author || '-'}</Descriptions.Item>
              <Descriptions.Item label="简介">{detailBook.description || '-'}</Descriptions.Item>
              <Descriptions.Item label="格式"><Tag>{detailBook.format}</Tag></Descriptions.Item>
              <Descriptions.Item label="语言">{detailBook.language || '-'}</Descriptions.Item>
              <Descriptions.Item label="章节">{detailBook.chapter_count}</Descriptions.Item>
              <Descriptions.Item label="文件名">{detailBook.filename}</Descriptions.Item>
              <Descriptions.Item label="文件大小">{formatFileSize(detailBook.file_size)}</Descriptions.Item>
              <Descriptions.Item label="SHA1"><Typography.Text copyable code>{detailBook.content_sha1}</Typography.Text></Descriptions.Item>
              <Descriptions.Item label="状态"><Tag color={STATUS_COLORS[detailBook.status]}>{detailBook.status}</Tag></Descriptions.Item>
              <Descriptions.Item label="来源">{detailBook.source}</Descriptions.Item>
              <Descriptions.Item label="上传时间">{detailBook.uploaded_at}</Descriptions.Item>
              <Descriptions.Item label="发布时间">{detailBook.published_at || '-'}</Descriptions.Item>
              <Descriptions.Item label="更新者">{detailBook.updated_by || '-'}</Descriptions.Item>
            </Descriptions>
          </div>
        )}
      </Modal>
    </>
  );
}
