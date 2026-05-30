import { Button, Card, Form, Input, Typography, message } from 'antd';
import { useNavigate } from 'react-router-dom';
import { login } from '../api/auth';
import { setAccessToken } from '../app/authStore';

export function LoginPage() {
  const navigate = useNavigate();

  async function handleFinish(values: { username: string; password: string }) {
    try {
      const result = await login(values.username, values.password);
      setAccessToken(result.access_token);
      navigate('/');
    } catch (error) {
      message.error(error instanceof Error ? error.message : '登录失败');
    }
  }

  return (
    <main style={{ minHeight: '100vh', display: 'grid', placeItems: 'center', background: '#f5f7fb' }}>
      <Card style={{ width: 360 }}>
        <Typography.Title level={3}>Oops Reader Manager</Typography.Title>
        <Form layout="vertical" onFinish={handleFinish}>
          <Form.Item name="username" label="用户名" rules={[{ required: true, message: '请输入用户名' }]}>
            <Input autoComplete="username" />
          </Form.Item>
          <Form.Item name="password" label="密码" rules={[{ required: true, message: '请输入密码' }]}>
            <Input.Password autoComplete="current-password" />
          </Form.Item>
          <Button type="primary" htmlType="submit" block>
            登录
          </Button>
        </Form>
      </Card>
    </main>
  );
}
