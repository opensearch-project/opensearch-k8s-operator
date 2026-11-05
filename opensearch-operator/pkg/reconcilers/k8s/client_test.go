package k8s

import (
	"context"
	"testing"

	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// The resource reconcilers write their status after every reconcile, and most
// reconciles change nothing, so an unchanged status must not be written.
func TestUdateObjectStatusSkipsUnchangedStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := opensearchv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name   string
		mutate func(*opensearchv1.OpensearchUser)
		writes int
		state  opensearchv1.OpensearchUserState
	}{
		{name: "unchanged status", mutate: func(u *opensearchv1.OpensearchUser) { u.Status.State = opensearchv1.OpensearchUserStateCreated }, writes: 0, state: opensearchv1.OpensearchUserStateCreated},
		{name: "changed status", mutate: func(u *opensearchv1.OpensearchUser) { u.Status.State = opensearchv1.OpensearchUserStateError }, writes: 1, state: opensearchv1.OpensearchUserStateError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			user := &opensearchv1.OpensearchUser{ObjectMeta: metav1.ObjectMeta{Name: "user", Namespace: "default"}}
			user.Status.State = opensearchv1.OpensearchUserStateCreated

			writes := 0
			c := interceptor.NewClient(
				fake.NewClientBuilder().WithScheme(scheme).WithObjects(user).WithStatusSubresource(user).Build(),
				interceptor.Funcs{
					SubResourceUpdate: func(ctx context.Context, c client.Client, subResourceName string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
						writes++
						return c.SubResource(subResourceName).Update(ctx, obj, opts...)
					},
				},
			)

			err := NewK8sClient(c, context.Background()).UdateObjectStatus(user, func(obj client.Object) {
				tc.mutate(obj.(*opensearchv1.OpensearchUser))
			})
			if err != nil {
				t.Fatalf("UdateObjectStatus() = %v", err)
			}
			if writes != tc.writes {
				t.Errorf("status writes = %d, want %d", writes, tc.writes)
			}

			stored := &opensearchv1.OpensearchUser{}
			if err := c.Get(context.Background(), client.ObjectKeyFromObject(user), stored); err != nil {
				t.Fatal(err)
			}
			if stored.Status.State != tc.state {
				t.Errorf("stored state = %q, want %q", stored.Status.State, tc.state)
			}
		})
	}
}
