export function shallowEqual(object1, object2, ignoreKeys = []) {
  const keys1 = Object.keys(object1).filter(key => !ignoreKeys.includes(key));
  const keys2 = Object.keys(object2).filter(key => !ignoreKeys.includes(key));

  if (keys1.length !== keys2.length) {
    return false;
  }

  for (let key of keys1) {
    if (object1[key] !== object2[key]) {
      return false;
    }
  }

  return true;
}
