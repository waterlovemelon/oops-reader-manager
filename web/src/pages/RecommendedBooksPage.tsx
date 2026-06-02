import { SearchOutlined, ReloadOutlined, PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons';
import { Button, Card, DatePicker, Form, Input, Modal, Select, Space, Table, Tag, message } from 'antd';
import type { Dayjs } from 'dayjs';
import dayjs from 'dayjs';
import { useCallback, useEffect, useRef, useState } from 'react';
import {
  BookRecommendation, CreateRecommendationInput, UpdateRecommendationInput,
  listRecommendations, createRecommendation, updateRecommendation, deleteRecommendation,
} from '../api/recommendations';
import { listBooks } from '../api/catalog';
import type { CatalogBook } from '../api/catalog';

const PUBLISH_STATE_CONFIG: Record<string, { color: string; label: string }> = {
  queued: { color: 'blue', label: '待发布' },
  published: { color: 'green', label: '已发布' },
  deleted: { color: 'red', label: '已删除' },
};

export function RecommendedBooksPage() {
  const [recommendations, setRecommendations] = useState<BookRecommendation[]>([]);
  const [loading, setLoading] = useState(false);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState('');
  const [statusFilter, setStatusFilter] = useState<string>('');

  // Modal state
  const [editRecord, setEditRecord] = useState<BookRecommendation | null>(null);
  const [modalOpen, setModalOpen] = useState(false);
  const [form] = Form.useForm();
  const [submitting, setSubmitting] = useState(false);

  // Book selector state
  const [bookOptions, setBookOptions] = useState<CatalogBook[]>([]);
  const [bookSearching, setBookSearching] = useState(false);
  const bookSearchTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Delete state
  const [deleteRecord, setDeleteRecord] = useState<BookRecommendation | null>(null);

  const fetchRecommendations = useCallback(() => {
    setLoading(true);
    listRecommendations(search || undefined, statusFilter || undefined, page)
      .then((res) => {
        setRecommendations(res.data);
        setTotal(res.pagination.total);
      })
      .catch(() => message.error('加载失败'))
      .finally(() => setLoading(false));
  }, [page, search, statusFilter]);

  useEffect(() => { fetchRecommendations(); }, [fetchRecommendations]);

  const handleSearch = () => {
    setPage(1);
    fetchRecommendations();
  };

  const handleRefresh = () => {
    setSearch('');
    setStatusFilter('');
    setPage(1);
  };

  // Debounced book search for the selector
  const handleBookSearch = useCallback((value: string) => {
    if (bookSearchTimer.current) clearTimeout(bookSearchTimer.current);
    if (!value.trim()) {
      setBookOptions([]);
      return;
    }
    bookSearchTimer.current = setTimeout(() => {
      setBookSearching(true);
      listBooks(value.trim(), undefined, 1, 20)
        .then((res) => setBookOptions(res.data))
        .catch(() => {})
        .finally(() => setBookSearching(false));
    }, 300);
  }, []);

  // Cleanup timer on unmount
  useEffect(() => {
    return () => { if (bookSearchTimer.current) clearTimeout(bookSearchTimer.current); };
  }, []);

  const openCreateModal = () => {
    setEditRecord(null);
    form.resetFields();
    setBookOptions([]);
    setModalOpen(true);
  };

  const openEditModal = (record: BookRecommendation) => {
    setEditRecord(record);
    form.setFieldsValue({
      book_key: record.book_key,
      comment: record.comment,
      scheduled_publish_at: record.scheduled_publish_at ? dayjs(record.scheduled_publish_at) : undefined,
    });
    // Pre-populate book options with the current book
    if (record.book) {
      setBookOptions([record.book]);
    }
    setModalOpen(true);
  };

  const handleModalOk = async () => {
    try {
      const values = await form.validateFields();
      setSubmitting(true);

      const scheduledAt: Dayjs | undefined = values.scheduled_publish_at;
      const payload: CreateRecommendationInput & UpdateRecommendationInput = {
        book_key: values.book_key,
        comment: values.comment || undefined,
        scheduled_publish_at: scheduledAt ? scheduledAt.toISOString() : undefined,
      };

      if (editRecord) {
        await updateRecommendation(editRecord.id, payload);
        message.success('更新成功');
      } else {
        await createRecommendation(payload);
        message.success('创建成功');
      }
      setModalOpen(false);
      fetchRecommendations();
    } catch (error) {
      if (error && typeof error === 'object' && 'errorFields' in error) return; // form validation
      message.error(error instanceof Error ? error.message : '操作失败');
    } finally {
      setSubmitting(false);
    }
  };

  const handleDelete = async () => {
    if (!deleteRecord) return;
    try {
      await deleteRecommendation(deleteRecord.id);
      message.success('已删除');
      setDeleteRecord(null);
      fetchRecommendations();
    } catch (error) {
      message.error(error instanceof Error ? error.message : '删除失败');
    }
  };

  return (
    <>
      <Card title="推荐书籍">
        <Space style={{ marginBottom: 16 }} wrap>
          <Input
            placeholder="搜索书名/作者/评论"
            prefix={<SearchOutlined />}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            onPressEnter={handleSearch}
            style={{ width: 240 }}
            allowClear
          />
          <Select
            placeholder="发布状态"
            value={statusFilter || undefined}
            onChange={(v) => { setStatusFilter(v ?? ''); setPage(1); }}
            allowClear
            style={{ width: 120 }}
            options={[
              { value: 'queued', label: '待发布' },
              { value: 'published', label: '已发布' },
              { value: 'deleted', label: '已删除' },
            ]}
          />
          <Button icon={<ReloadOutlined />} onClick={handleRefresh}>刷新</Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreateModal}>新增推荐</Button>
        </Space>

        <Table
          rowKey="id"
          loading={loading}
          dataSource={recommendations}
          pagination={{
            current: page,
            pageSize: 20,
            total,
            onChange: setPage,
            showTotal: (t) => `共 ${t} 条`,
          }}
          columns={[
            {
              title: '书籍',
              key: 'book',
              width: 200,
              render: (_: unknown, record: BookRecommendation) => {
                const book = record.book;
                if (!book) return record.book_key;
                return (
                  <div>
                    <div style={{ fontWeight: 500 }}>{book.title}</div>
                    <div style={{ color: '#999', fontSize: 12 }}>{book.author || '-'}</div>
                  </div>
                );
              },
            },
            {
              title: '评论',
              dataIndex: 'comment',
              ellipsis: true,
              render: (v: string) => (
                <div style={{ display: '-webkit-box', WebkitLineClamp: 2, WebkitBoxOrient: 'vertical', overflow: 'hidden' }}>
                  {v || '-'}
                </div>
              ),
            },
            {
              title: '发布状态',
              dataIndex: 'publish_state',
              width: 100,
              render: (v: string) => {
                const cfg = PUBLISH_STATE_CONFIG[v] ?? { color: 'default', label: v };
                return <Tag color={cfg.color}>{cfg.label}</Tag>;
              },
            },
            {
              title: '定时发布',
              dataIndex: 'scheduled_publish_at',
              width: 170,
              render: (v: string | null) => v ? dayjs(v).format('YYYY-MM-DD HH:mm') : '-',
            },
            {
              title: '创建者',
              dataIndex: 'created_by',
              width: 100,
              ellipsis: true,
            },
            {
              title: '操作',
              width: 120,
              render: (_: unknown, record: BookRecommendation) => (
                <Space size="small">
                  <Button size="small" icon={<EditOutlined />} onClick={() => openEditModal(record)}>编辑</Button>
                  <Button size="small" danger icon={<DeleteOutlined />} onClick={() => setDeleteRecord(record)}>删除</Button>
                </Space>
              ),
            },
          ]}
        />
      </Card>

      {/* Create / Edit Modal */}
      <Modal
        title={editRecord ? '编辑推荐' : '新增推荐'}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={handleModalOk}
        confirmLoading={submitting}
        destroyOnHidden
        width={520}
      >
        <Form form={form} layout="vertical">
          <Form.Item
            name="book_key"
            label="书籍"
            rules={[{ required: true, message: '请选择书籍' }]}
          >
            <Select
              showSearch
              placeholder="输入书名搜索..."
              filterOption={false}
              onSearch={handleBookSearch}
              loading={bookSearching}
              notFoundContent={bookSearching ? '搜索中...' : '无结果'}
              options={bookOptions.map((b) => ({
                value: b.book_key,
                label: `${b.title}${b.author ? ` - ${b.author}` : ''}`,
              }))}
            />
          </Form.Item>
          <Form.Item
            name="comment"
            label="评论"
            rules={[{ max: 2000, message: '评论不超过 2000 字' }]}
          >
            <Input.TextArea rows={4} maxLength={2000} showCount placeholder="推荐理由..." />
          </Form.Item>
          <Form.Item
            name="scheduled_publish_at"
            label="定时发布"
            tooltip="不设置则立即发布"
          >
            <DatePicker
              showTime
              format="YYYY-MM-DD HH:mm"
              style={{ width: '100%' }}
              placeholder="选择发布时间"
            />
          </Form.Item>
          <div style={{ color: '#999', fontSize: 12, marginTop: -8 }}>
            不设置定时发布时间将立即发布该推荐。
          </div>
        </Form>
      </Modal>

      {/* Delete Confirm Modal */}
      <Modal
        title="确认删除"
        open={!!deleteRecord}
        onCancel={() => setDeleteRecord(null)}
        onOk={handleDelete}
        okText="删除"
        okButtonProps={{ danger: true }}
        destroyOnHidden
      >
        <p>确定要删除这条推荐吗？删除后将标记为已删除状态，可在列表中通过状态筛选查看。</p>
        {deleteRecord?.book && (
          <p>书籍：《{deleteRecord.book.title}》</p>
        )}
      </Modal>
    </>
  );
}
