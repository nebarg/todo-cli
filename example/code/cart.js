// todo00 totals lose a cent on some discounts
export function total(items, discount) {
  const sum = items.reduce((acc, item) => acc + item.price * item.quantity, 0);
  return sum - sum * discount;
}

/*
 * TODO: keep the cart between visits
 */
export function addItem(cart, item) {
  cart.items.push(item); // todo2 merge duplicate items
  return cart;
}
