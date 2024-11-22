namespace Service;
public class UserState
{
    public List<User> Users { get; private set; } = new();
    public List<string> hasMsgList = new List<string>();
    public event Action? OnChange;

    public void AddHasMsg(string id){
        hasMsgList.Add(id);
        NotifyStateChanged();
    }
    public void RemoveHasMsg(string id){
        hasMsgList.Remove(id);
        NotifyStateChanged();
    }
    public void UpdateUsers(List<User> users)
    {
        Users = users;
        NotifyStateChanged();
    }
    public User GetUserById(string id){
        // 在用户列表中查找与给定 id 匹配的 User
        var user = Users.FirstOrDefault(u => u.Id == id);
        return user??new User(); // 如果未找到，FirstOrDefault 返回 null
    }

    private void NotifyStateChanged() => OnChange?.Invoke();
}


