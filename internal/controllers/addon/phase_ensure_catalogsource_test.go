package addon

import (
	"context"
	"testing"

	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	k8sApiErrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	addonsv1alpha1 "github.com/openshift/addon-operator/apis/addons/v1alpha1"

	"github.com/openshift/addon-operator/internal/controllers"
	"github.com/openshift/addon-operator/internal/testutil"
)

func TestReconcileCatalogSource_NotExistingYet_HappyPath(t *testing.T) {
	c := testutil.NewClient()
	c.On("Get",
		testutil.IsContext,
		testutil.IsObjectKey,
		testutil.IsOperatorsV1Alpha1CatalogSourcePtr,
	).Return(testutil.NewTestErrNotFound())
	c.On("Create",
		testutil.IsContext,
		testutil.IsOperatorsV1Alpha1CatalogSourcePtr,
		mock.Anything,
	).Return(nil)

	ctx := context.Background()
	catalogSource := testutil.NewTestCatalogSource()
	reconciledCatalogSource, err := reconcileCatalogSource(ctx, c, catalogSource.DeepCopy(), addonsv1alpha1.ResourceAdoptionAdoptAll)
	assert.NoError(t, err)
	assert.NotNil(t, reconciledCatalogSource)
	c.AssertExpectations(t)
	c.AssertCalled(t, "Get", testutil.IsContext, client.ObjectKey{
		Name:      catalogSource.Name,
		Namespace: catalogSource.Namespace,
	}, testutil.IsOperatorsV1Alpha1CatalogSourcePtr)
	c.AssertCalled(t, "Create", testutil.IsContext,
		testutil.IsOperatorsV1Alpha1CatalogSourcePtr, mock.Anything)
}

func TestReconcileCatalogSource_NotExistingYet_WithClientErrorGet(t *testing.T) {
	timeoutErr := k8sApiErrors.NewTimeoutError("for testing", 1)

	c := testutil.NewClient()
	c.On("Get",
		testutil.IsContext,
		testutil.IsObjectKey,
		testutil.IsOperatorsV1Alpha1CatalogSourcePtr,
	).Return(timeoutErr)

	ctx := context.Background()
	_, err := reconcileCatalogSource(ctx, c, testutil.NewTestCatalogSource(), addonsv1alpha1.ResourceAdoptionAdoptAll)
	assert.Error(t, err)
	assert.EqualError(t, err, timeoutErr.Error())
	c.AssertExpectations(t)
}

func TestReconcileCatalogSource_NotExistingYet_WithClientErrorCreate(t *testing.T) {
	timeoutErr := k8sApiErrors.NewTimeoutError("for testing", 1)

	c := testutil.NewClient()
	c.On("Get",
		testutil.IsContext,
		testutil.IsObjectKey,
		testutil.IsOperatorsV1Alpha1CatalogSourcePtr,
	).Return(testutil.NewTestErrNotFound())
	c.On("Create",
		testutil.IsContext,
		testutil.IsOperatorsV1Alpha1CatalogSourcePtr,
		mock.Anything,
	).Return(timeoutErr)

	ctx := context.Background()
	_, err := reconcileCatalogSource(ctx, c, testutil.NewTestCatalogSource(), addonsv1alpha1.ResourceAdoptionAdoptAll)
	assert.Error(t, err)
	assert.EqualError(t, err, timeoutErr.Error())
	c.AssertExpectations(t)
}

func TestReconcileCatalogSource_Adoption_AdoptAll(t *testing.T) {
	catalogSource := testutil.NewTestCatalogSource()

	c := testutil.NewClient()
	c.On("Get",
		testutil.IsContext,
		testutil.IsObjectKey,
		testutil.IsOperatorsV1Alpha1CatalogSourcePtr,
	).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*operatorsv1alpha1.CatalogSource)
		testutil.NewTestCatalogSourceWithoutOwner().DeepCopyInto(arg)
	}).Return(nil)
	// TODO: remove this Update call once resourceAdoptionStrategy is discontinued
	// This update call changes the ownerRef to AddonOperator
	c.On("Update",
		testutil.IsContext,
		testutil.IsOperatorsV1Alpha1CatalogSourcePtr,
		mock.Anything,
	).Return(nil)

	ctx := context.Background()
	reconciledCatalogSource, err := reconcileCatalogSource(ctx, c, catalogSource.DeepCopy(), addonsv1alpha1.ResourceAdoptionAdoptAll)
	assert.NoError(t, err)
	assert.NotNil(t, reconciledCatalogSource)
}

func TestReconcileCatalogSource_Adoption_Prevent(t *testing.T) {
	catalogSource := testutil.NewTestCatalogSource()

	c := testutil.NewClient()
	c.On("Get",
		testutil.IsContext,
		testutil.IsObjectKey,
		testutil.IsOperatorsV1Alpha1CatalogSourcePtr,
	).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*operatorsv1alpha1.CatalogSource)
		testutil.NewTestCatalogSourceWithoutOwner().DeepCopyInto(arg)
	}).Return(nil)

	ctx := context.Background()
	_, err := reconcileCatalogSource(ctx, c, catalogSource.DeepCopy(), addonsv1alpha1.ResourceAdoptionPrevent)

	assert.Error(t, err)
	assert.EqualError(t, err, controllers.ErrNotOwnedByUs.Error())
	c.AssertExpectations(t)
}

func TestEnsureCatalogSource_Create(t *testing.T) {
	addon := testutil.NewTestAddonWithCatalogSourceImage()

	c := testutil.NewClient()
	c.On("Get",
		testutil.IsContext,
		testutil.IsObjectKey,
		testutil.IsOperatorsV1Alpha1CatalogSourcePtr,
	).Return(testutil.NewTestErrNotFound())
	c.On("Create",
		testutil.IsContext,
		testutil.IsOperatorsV1Alpha1CatalogSourcePtr,
		mock.Anything,
	).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*operatorsv1alpha1.CatalogSource)
		arg.Status.GRPCConnectionState = &operatorsv1alpha1.GRPCConnectionState{
			LastObservedState: "READY",
		}
	}).Return(nil)

	r := &AddonReconciler{
		Client: c,
		Log:    testutil.NewLogger(t),
		Scheme: testutil.NewTestSchemeWithAddonsv1alpha1(),
	}

	log := testutil.NewLogger(t)

	ctx := context.Background()
	requeueResult, _, err := r.ensureCatalogSource(ctx, log, addon)
	assert.NoError(t, err)
	assert.Equal(t, resultNil, requeueResult)
	c.AssertExpectations(t)
}

func TestEnsureCatalogSource_Update(t *testing.T) {
	addon := testutil.NewTestAddonWithCatalogSourceImageWithResourceAdoptionStrategy(addonsv1alpha1.ResourceAdoptionAdoptAll)

	c := testutil.NewClient()
	c.On("Get",
		testutil.IsContext,
		testutil.IsObjectKey,
		testutil.IsOperatorsV1Alpha1CatalogSourcePtr,
	).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*operatorsv1alpha1.CatalogSource)
		arg.Status.GRPCConnectionState = &operatorsv1alpha1.GRPCConnectionState{
			LastObservedState: "READY",
		}
	}).Return(nil)
	c.On("Update",
		testutil.IsContext,
		testutil.IsOperatorsV1Alpha1CatalogSourcePtr,
		mock.Anything,
	).Return(nil)

	r := &AddonReconciler{
		Client: c,
		Log:    testutil.NewLogger(t),
		Scheme: testutil.NewTestSchemeWithAddonsv1alpha1(),
	}

	log := testutil.NewLogger(t)

	ctx := context.Background()
	requeueResult, _, err := r.ensureCatalogSource(ctx, log, addon)
	assert.NoError(t, err)
	assert.Equal(t, resultNil, requeueResult)
	c.AssertExpectations(t)
	c.AssertNumberOfCalls(t, "Get", 1)
	c.AssertNumberOfCalls(t, "Update", 1)
}