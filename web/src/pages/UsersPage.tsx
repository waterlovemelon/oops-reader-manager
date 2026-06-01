import { SearchOutlined } from '@ant-design/icons';
import { Button, Card, DatePicker, Drawer, Form, Input, Modal, Select, Space, Table, Tag, message } from 'antd';
import { useEffect, useState } from 'react';
import { User, listUsers, updateUserStatus } from '../api/users';
import { Entitlement, listEntitlements, createEntitlement, revokeEntitlement, extendEntitlement } from '../api/entitlements';

const STATUS_COLORS: Record<string, string> = {
  active: 'green',
  frozen: 'red',
};

const ENTITLEMENT_STATUS_COLORS: Record<string, string> = {
  active: 'green',
  revoked: 'red',
  inactive: 'default',
};

const ENTITLEMENT_LABELS: Record<string, string> = {
  vip: 'VIP',
  tts_premium: 'TTS 高级',
  cloud_backup_plus: '云备份增强',
};

export function UsersPage() {
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(false);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState('');
  const [drawerUser, setDrawerUser] = useState<User | null>(null);

  const fetchUsers = () => {
    setLoading(true);
    listUsers(search || undefined, page)
      .then((res) => {
        setUsers(res.data);
        setTotal(res.pagination.total);
      })
      .catch(() => message.error('加载失败'))
      .finally(() => setLoading(false));
  };

  useEffect(() => { fetchUsers(); }, [page]);

  const handleSearch = () => {
    setPage(1);
    fetchUsers();
  };

  const handleStatusChange = async (id: string, newStatus: string) => {
    try {
      await updateUserStatus(id, newStatus);
      message.success(newStatus === 'frozen' ? '用户已冻结' : '用户已解冻');
      fetchUsers();
    } catch (error) {
      message.error(error instanceof Error ? error.message : '操作失败');
    }
  };

  return (
    <>
      <Card title="用户管理">
        <Space style={{ marginBottom: 16 }}>
          <Input
            placeholder="搜索邮箱/昵称/ID"
            prefix={<SearchOutlined />}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            onPressEnter={handleSearch}
            style={{ width: 280 }}
            allowClear
          />
          <Button onClick={handleSearch}>搜索</Button>
        </Space>
        <Table
          rowKey="id"
          loading={loading}
          dataSource={users}
          pagination={{
            current: page,
            pageSize: 20,
            total,
            onChange: setPage,
            showTotal: (t) => `共 ${t} 人`,
          }}
          columns={[
            { title: 'ID', dataIndex: 'id', width: 80 },
            { title: '邮箱', dataIndex: 'email', ellipsis: true },
            { title: '昵称', dataIndex: 'display_name', ellipsis: true },
            {
              title: '状态', dataIndex: 'status', width: 100,
              render: (v: string) => <Tag color={STATUS_COLORS[v] ?? 'default'}>{v}</Tag>,
            },
            { title: '创建时间', dataIndex: 'created_at', width: 170 },
            {
              title: '操作', width: 200,
              render: (_: unknown, record: User) => (
                <Space size="small">
                  <Button size="small" onClick={() => setDrawerUser(record)}>权益</Button>
                  {record.status === 'active' ? (
                    <Button size="small" danger onClick={() => {
                      Modal.confirm({
                        title: '确认冻结？',
                        content: `确定要冻结用户 ${record.email || record.id} 吗？冻结后该用户将无法登录。`,
                        onOk: () => handleStatusChange(record.id, 'frozen'),
                      });
                    }}>冻结</Button>
                  ) : (
                    <Button size="small" type="primary" onClick={() => handleStatusChange(record.id, 'active')}>解冻</Button>
                  )}
                </Space>
              ),
            },
          ]}
        />
      </Card>

      {drawerUser && (
        <EntitlementDrawer
          user={drawerUser}
          onClose={() => setDrawerUser(null)}
        />
      )}
    </>
  );
}

function EntitlementDrawer({ user, onClose }: { user: User; onClose: () => void }) {
  const [entitlements, setEntitlements] = useState<Entitlement[]>([]);
  const [loading, setLoading] = useState(false);
  const [showCreate, setShowCreate] = useState(false);
  const [createForm] = Form.useForm();
  const [extendId, setExtendId] = useState<number | null>(null);
  const [extendForm] = Form.useForm();

  const fetchEntitlements = () => {
    setLoading(true);
    listEntitlements(user.id)
      .then((res) => setEntitlements(res.data))
      .catch(() => message.error('加载权益失败'))
      .finally(() => setLoading(false));
  };

  useEffect(() => { fetchEntitlements(); }, [user.id]);

  const handleCreate = async () => {
    try {
      const values = await createForm.validateFields();
      await createEntitlement(user.id, {
        entitlement_key: values.entitlement_key,
        starts_at: values.starts_at.toISOString(),
        expires_at: values.expires_at?.toISOString(),
      });
      message.success('权益已发放');
      setShowCreate(false);
      createForm.resetFields();
      fetchEntitlements();
    } catch (error) {
      message.error(error instanceof Error ? error.message : '发放失败');
    }
  };

  const handleRevoke = async (id: number) => {
    try {
      await revokeEntitlement(id);
      message.success('权益已撤销');
      fetchEntitlements();
    } catch (error) {
      message.error(error instanceof Error ? error.message : '撤销失败');
    }
  };

  const handleExtend = async () => {
    if (extendId === null) return;
    try {
      const values = await extendForm.validateFields();
      await extendEntitlement(extendId, values.expires_at.toISOString());
      message.success('延期成功');
      setExtendId(null);
      extendForm.resetFields();
      fetchEntitlements();
    } catch (error) {
      message.error(error instanceof Error ? error.message : '延期失败');
    }
  };

  return (
    <Drawer
      title={`权益管理 — ${user.email || user.display_name || user.id}`}
      open
      onClose={onClose}
      width={600}
      extra={
        <Button type="primary" onClick={() => setShowCreate(true)}>发放权益</Button>
      }
    >
      <Table
        rowKey="id"
        loading={loading}
        dataSource={entitlements}
        pagination={false}
        columns={[
          {
            title: '权益', dataIndex: 'entitlement_key',
            render: (v: string) => ENTITLEMENT_LABELS[v] ?? v,
          },
          {
            title: '状态', dataIndex: 'status', width: 80,
            render: (v: string) => <Tag color={ENTITLEMENT_STATUS_COLORS[v] ?? 'default'}>{v}</Tag>,
          },
          { title: '来源', dataIndex: 'source', width: 80 },
          { title: '开始', dataIndex: 'starts_at', width: 170 },
          {
            title: '到期', dataIndex: 'expires_at', width: 170,
            render: (v: string | null) => v ?? '永久',
          },
          {
            title: '操作', width: 150,
            render: (_: unknown, record: Entitlement) => (
              record.status === 'active' ? (
                <Space size="small">
                  <Button size="small" onClick={() => setExtendId(record.id)}>延期</Button>
                  <Button size="small" danger onClick={() => {
                    Modal.confirm({
                      title: '确认撤销？',
                      content: `确定要撤销 ${ENTITLEMENT_LABELS[record.entitlement_key] ?? record.entitlement_key} 吗？`,
                      onOk: () => handleRevoke(record.id),
                    });
                  }}>撤销</Button>
                </Space>
              ) : null
            ),
          },
        ]}
      />

      <Modal
        title="发放权益"
        open={showCreate}
        onCancel={() => { setShowCreate(false); createForm.resetFields(); }}
        onOk={handleCreate}
        destroyOnClose
      >
        <Form form={createForm} layout="vertical">
          <Form.Item name="entitlement_key" label="权益类型" rules={[{ required: true }]}>
            <Select options={[
              { value: 'vip', label: 'VIP' },
              { value: 'tts_premium', label: 'TTS 高级' },
              { value: 'cloud_backup_plus', label: '云备份增强' },
            ]} />
          </Form.Item>
          <Form.Item name="starts_at" label="开始时间" rules={[{ required: true }]}>
            <DatePicker showTime style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="expires_at" label="到期时间（留空为永久）">
            <DatePicker showTime style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="延期"
        open={extendId !== null}
        onCancel={() => { setExtendId(null); extendForm.resetFields(); }}
        onOk={handleExtend}
        destroyOnClose
      >
        <Form form={extendForm} layout="vertical">
          <Form.Item name="expires_at" label="新到期时间" rules={[{ required: true }]}>
            <DatePicker showTime style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>
    </Drawer>
  );
}
